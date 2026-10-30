package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PreviewDrain freezes identities and safety-relevant fields, performs eviction
// dry runs and prepares the shared cordon Diff. It never evicts or cordons.
func (s *Service) PreviewDrain(ctx context.Context, request DrainRequest) (Plan, error) {
	if err := validateTarget(request.Target); err != nil {
		return Plan{}, err
	}
	if request.Options.TimeoutSeconds < 30 || request.Options.TimeoutSeconds > 3600 {
		return Plan{}, errors.New("排空超时必须在 30 至 3600 秒之间")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client, err := s.resolver.NodeClient(request.Target.ClusterID)
	if err != nil {
		return Plan{}, err
	}
	pods, err := collectPods(ctx, client, request.Target)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Pods: []PodProgress{}}
	fingerprints := map[string]string{}
	blocked := false
	for _, object := range pods {
		pod := summarizePod(object)
		row := classify(pod, request.Options)
		if row.State == "pending" {
			result, err := client.EvictPod(ctx, Eviction{Target: podTarget(request.Target, pod), Baseline: formedit.Baseline(object), DryRun: true})
			if err != nil {
				row.State = "blocked"
				row.Message = err.Error()
			} else if result.Waiting {
				row.Message = "预检受 PDB 或 API 限流阻塞；执行时等待，不绕过保护"
			} else {
				row.Message = "驱逐 Dry Run 通过；执行时仍由集群重新校验"
			}
		}
		if row.State == "blocked" {
			blocked = true
		}
		digest, err := fingerprint(object)
		if err != nil {
			return Plan{}, err
		}
		fingerprints[pod.UID] = digest
		plan.Pods = append(plan.Pods, row)
	}
	cordon, err := s.PreviewCordon(ctx, CordonRequest{Target: request.Target, Baseline: request.Baseline, Unschedulable: true})
	if err != nil {
		return Plan{}, err
	}
	plan.Cordon = cordon
	plan.Cordon.Token = ""
	retained := false
	defer func() {
		if !retained {
			s.writer.Discard(cordon.Token)
		}
	}()
	if blocked {
		return plan, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || ctx.Err() != nil {
		return Plan{}, errors.New("排空预检已取消")
	}
	if len(s.plans) >= 8 {
		return Plan{}, errors.New("待确认排空过多，请关闭其他预检")
	}
	plan.Token = uuid.NewString()
	expires := time.Now().Add(planTTL)
	plan.ExpiresAt = expires.Format(time.RFC3339)
	s.plans[plan.Token] = pendingPlan{request: request, plan: plan, fingerprints: fingerprints, connectionID: client.ConnectionID(), cordonToken: cordon.Token, expires: expires}
	retained = true
	return plan, nil
}

func classify(pod Pod, options DrainOptions) PodProgress {
	row := PodProgress{Pod: pod, State: "pending"}
	switch pod.Category {
	case "static":
		row.State = "skipped"
		row.Message = "静态 Pod 的镜像对象，保留并由 kubelet 管理"
		return row
	case "daemonset":
		row.State = "blocked"
		row.Message = "DaemonSet Pod，需要明确选择保留"
		if options.IgnoreDaemonSets {
			row.State = "skipped"
			row.Message = "保留 DaemonSet Pod"
		}
		return row
	}
	if pod.Terminating {
		row.State = "waiting"
		row.Message = "已经在删除，等待原 UID 消失"
		return row
	}
	if pod.Category == "unmanaged" && pod.Phase != "Succeeded" && pod.Phase != "Failed" && !options.AllowUnmanaged {
		row.State = "blocked"
		row.Message = "无控制器引用，驱逐后可能不会重建"
	}
	if len(pod.LocalData) > 0 && !options.AllowLocalData {
		row.State = "blocked"
		row.Message = "含 emptyDir / hostPath，本地数据可能丢失或无法随 Pod 迁移"
	}
	return row
}

func collectPods(ctx context.Context, client Reader, target resource.Ref) ([]*unstructured.Unstructured, error) {
	var result []*unstructured.Unstructured
	seen := map[string]bool{}
	cursors := map[string]bool{}
	cursor := ""
	for i := 0; i < maxPods/100; i++ {
		page, err := listPods(ctx, client, target, "", cursor)
		if err != nil {
			return nil, err
		}
		for j := range page.Items {
			object := &page.Items[j]
			uid := string(object.GetUID())
			if seen[uid] {
				return nil, errors.New("Pod 分页重复，请重新预检")
			}
			seen[uid] = true
			result = append(result, object)
		}
		cursor = page.GetContinue()
		if cursor == "" {
			sort.Slice(result, func(i, j int) bool {
				return result[i].GetNamespace()+"/"+result[i].GetName() < result[j].GetNamespace()+"/"+result[j].GetName()
			})
			return result, nil
		}
		if cursors[cursor] {
			return nil, errors.New("Pod 分页游标重复")
		}
		cursors[cursor] = true
	}
	return nil, errors.New("节点 Pod 超过 500 个，当前无法完整预检，不会启动排空")
}

// Status changes are expected during drain. Spec, labels, ownership and annotations
// are frozen so a reviewed Pod cannot silently change policy or controller.
func fingerprint(object *unstructured.Unstructured) (string, error) {
	data, err := json.Marshal([]interface{}{object.Object["spec"], object.GetLabels(), object.GetAnnotations(), object.GetOwnerReferences()})
	if err != nil {
		return "", errors.New("Pod 内容不能序列化，无法冻结排空清单")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func checkInventory(plan pendingPlan, pods []*unstructured.Unstructured) error {
	for _, object := range pods {
		before, ok := plan.fingerprints[string(object.GetUID())]
		digest, err := fingerprint(object)
		if err != nil {
			return err
		}
		if !ok || before != digest {
			return errors.New("节点出现新 Pod 或 Pod 配置/归属已变化，请重新预检；不会驱逐未确认的对象")
		}
	}
	return nil
}
