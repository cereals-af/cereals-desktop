package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/metrics"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const metricsResponseLimit = 8 << 20

// metricsDataError carries only local validation messages safe for the UI.
type metricsDataError string

func (err metricsDataError) Error() string { return string(err) }

type resourceMetric struct {
	Metadata   metav1.ObjectMeta   `json:"metadata"`
	Timestamp  metav1.Time         `json:"timestamp"`
	Window     metav1.Duration     `json:"window"`
	Usage      corev1.ResourceList `json:"usage"`
	Containers []struct {
		Name  string              `json:"name"`
		Usage corev1.ResourceList `json:"usage"`
	} `json:"containers"`
}

func (c *Client) ResourceUsage(ctx context.Context, query metrics.Query) (metrics.Snapshot, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	rows, err := c.readMetrics(ctx, query)
	if err != nil {
		return metricsFailure(ctx, err)
	}
	identities, err := c.metricIdentities(ctx, query)
	if err != nil {
		result, failure := metricsFailure(ctx, err)
		result.Message = "无法校验资源身份：" + result.Message
		return result, failure
	}
	byName := make(map[string]resourceMetric, len(rows))
	for _, row := range rows {
		byName[row.Metadata.Namespace+"/"+row.Metadata.Name] = row
	}
	result := metrics.Snapshot{State: "available", Items: make([]metrics.Sample, 0, len(query.Targets))}
	now := time.Now()
	for _, target := range query.Targets {
		key := target.Namespace + "/" + target.Name
		row, exists := byName[key]
		result.Items = append(result.Items, metricSample(target, identities[key], row, exists, query.Resource, now))
	}
	return result, nil
}

func (c *Client) readMetrics(ctx context.Context, query metrics.Query) ([]resourceMetric, error) {
	var group metav1.APIGroup
	if err := c.readMetricsJSON(ctx, "/apis/metrics.k8s.io", &group); err != nil {
		return nil, err
	}
	version := ""
	for _, candidate := range append([]metav1.GroupVersionForDiscovery{group.PreferredVersion}, group.Versions...) {
		if candidate.Version == "v1" || candidate.Version == "v1beta1" {
			version = candidate.Version
			break
		}
	}
	if version == "" {
		return nil, metricsDataError("Metrics API 未提供支持的版本")
	}
	path := "/apis/metrics.k8s.io/" + version
	if query.Namespace != "" {
		path += "/namespaces/" + query.Namespace
	}
	var list struct {
		Items []resourceMetric `json:"items"`
	}
	if err := c.readMetricsJSON(ctx, path+"/"+query.Resource, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *Client) readMetricsJSON(ctx context.Context, path string, value any) error {
	stream, err := c.rest.Get().AbsPath(path).SetHeader("Accept", "application/json").Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, metricsResponseLimit+1))
	if err != nil {
		return err
	}
	if len(data) > metricsResponseLimit {
		return metricsDataError("指标响应超过 8 MiB，请缩小命名空间范围")
	}
	if err := json.Unmarshal(data, value); err != nil {
		return metricsDataError("Metrics API 返回了无效的指标数据")
	}
	return nil
}

// Metrics often omit UID. Read current metadata before joining by name to avoid
// displaying a replacement Pod's sample beside the previous object's identity.
func (c *Client) metricIdentities(ctx context.Context, query metrics.Query) (map[string]metav1.PartialObjectMetadata, error) {
	api := c.metadata.Resource(schema.GroupVersionResource{Version: "v1", Resource: query.Resource}).Namespace(query.Namespace)
	result := make(map[string]metav1.PartialObjectMetadata, len(query.Targets))
	wanted := make(map[string]bool, len(query.Targets))
	for _, target := range query.Targets {
		wanted[target.Namespace+"/"+target.Name] = true
	}
	options := metav1.ListOptions{Limit: 500}
	for scanned := 0; scanned < 10000; {
		page, err := api.List(ctx, options)
		if err != nil {
			return nil, err
		}
		scanned += len(page.Items)
		for _, item := range page.Items {
			key := item.Namespace + "/" + item.Name
			if wanted[key] {
				result[key] = item
			}
		}
		if page.Continue == "" || len(result) == len(wanted) {
			return result, nil
		}
		if options.Continue == page.Continue {
			return nil, metricsDataError("资源身份校验分页未推进")
		}
		options.Continue = page.Continue
	}
	return nil, metricsDataError("资源身份校验超过 10000 项，请缩小查询范围")
}

func metricSample(target metrics.Target, identity metav1.PartialObjectMetadata, row resourceMetric, exists bool, kind string, now time.Time) metrics.Sample {
	result := metrics.Sample{Target: target, State: "missing", Message: "暂无采样，容器可能尚未启动或已经结束"}
	if string(identity.UID) != target.UID {
		result.State, result.Message = "replaced", "资源已删除或重建，请刷新资源列表"
		return result
	}
	if !exists {
		return result
	}
	result.Timestamp, result.Window = row.Timestamp.UTC().Format(time.RFC3339), row.Window.Duration.String()
	if row.Timestamp.IsZero() || row.Timestamp.Before(&identity.CreationTimestamp) || (row.Metadata.UID != "" && row.Metadata.UID != identity.UID) {
		result.State, result.Message = "missing", "等待当前资源的有效采样"
		return result
	}
	if now.Sub(row.Timestamp.Time) > 2*time.Minute || row.Timestamp.Time.After(now.Add(time.Minute)) {
		result.State, result.Message = "stale", "采样已过期或集群时钟异常"
		return result
	}
	usage, err := metricUsage(row, kind)
	if err != nil {
		result.State, result.Message = "invalid", err.Error()
		return result
	}
	result.State, result.Message, result.Usage = "available", "", &usage
	return result
}

func metricUsage(row resourceMetric, kind string) (metrics.Usage, error) {
	parts := []corev1.ResourceList{row.Usage}
	if kind == "pods" {
		parts = nil
		for _, container := range row.Containers {
			parts = append(parts, container.Usage)
		}
	}
	if len(parts) == 0 {
		return metrics.Usage{}, errors.New("尚无容器用量采样")
	}
	result := metrics.Usage{}
	for _, part := range parts {
		cpu, hasCPU := part[corev1.ResourceCPU]
		memory, hasMemory := part[corev1.ResourceMemory]
		if !hasCPU || !hasMemory || cpu.Sign() < 0 || memory.Sign() < 0 {
			return metrics.Usage{}, errors.New("CPU 或内存采样不完整")
		}
		result.CPUCores += cpu.AsApproximateFloat64()
		result.MemoryBytes += memory.AsApproximateFloat64()
	}
	if math.IsInf(result.CPUCores, 0) || math.IsInf(result.MemoryBytes, 0) || math.IsNaN(result.CPUCores) || math.IsNaN(result.MemoryBytes) {
		return metrics.Usage{}, errors.New("指标数值超出支持范围")
	}
	return result, nil
}

func metricsFailure(ctx context.Context, err error) (metrics.Snapshot, error) {
	if ctx.Err() != nil {
		return metrics.Snapshot{}, fmt.Errorf("资源用量查询已取消或超时: %w", ctx.Err())
	}
	state, message := "error", requestError(err).Error()
	var dataError metricsDataError
	switch {
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		state, message = "forbidden", "当前身份无权读取资源用量"
	case apierrors.IsNotFound(err), apierrors.IsServiceUnavailable(err):
		state, message = "unavailable", "Metrics Server 未安装或暂不可用"
	case errors.As(err, &dataError):
		message = dataError.Error()
	}
	return metrics.Snapshot{State: state, Message: message, Items: []metrics.Sample{}}, nil
}
