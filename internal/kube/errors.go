package kube

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"strings"
	"unicode"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// requestError intentionally excludes raw response bodies and exec-plugin stderr:
// either may contain tokens or other credentials supplied by an external system.
func requestError(err error) error {
	var unknownCA x509.UnknownAuthorityError
	var invalidCert x509.CertificateInvalidError
	var hostname x509.HostnameError
	var network net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return errors.New("请求已取消")
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("连接超时，请检查网络、VPN 或 API Server 地址")
	case apierrors.IsUnauthorized(err):
		return errors.New("认证失败：凭据无效或已过期，请更新 kubeconfig 或认证插件")
	case apierrors.IsForbidden(err):
		return errors.New("权限不足：当前身份无权执行此操作，可尝试指定有权限的命名空间")
	case apierrors.IsConflict(err):
		return errors.New("资源版本或 SSA 字段归属冲突：请保留草稿，重新读取并比较；不会强制覆盖其他管理者")
	case apierrors.IsAlreadyExists(err):
		return errors.New("同名资源已存在，请重新读取后选择更新")
	case apierrors.IsInvalid(err):
		return validationError(err)
	case apierrors.IsBadRequest(err):
		return errors.New("请求未通过服务端校验：请检查未知字段、字段类型和资源 Schema")
	case apierrors.IsNotFound(err):
		return errors.New("资源或 API 不存在，可能已删除或当前集群不支持")
	case apierrors.IsResourceExpired(err):
		return errors.New("分页快照已过期，请刷新后重新加载")
	case apierrors.IsTooManyRequests(err):
		return errors.New("API Server 正在限流，请稍后重试")
	case errors.As(err, &unknownCA), errors.As(err, &invalidCert), errors.As(err, &hostname):
		return errors.New("TLS 证书校验失败，请检查 CA、证书有效期和服务器名称")
	case errors.As(err, &network):
		return errors.New("网络请求失败，请检查 API Server、DNS、VPN 和代理配置")
	default:
		return errors.New("Kubernetes 请求失败，请检查连接配置和认证插件状态")
	}
}

func validationError(err error) error {
	var status apierrors.APIStatus
	fields := []string{}
	if errors.As(err, &status) && status.Status().Details != nil {
		for _, cause := range status.Status().Details.Causes {
			// Never return rejected values or admission messages; they can contain Secrets.
			field := strings.Map(func(r rune) rune {
				if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._[]-/", r) {
					return r
				}
				return -1
			}, cause.Field)
			if len(field) > 120 {
				field = field[:120]
			}
			if field != "" {
				fields = append(fields, field+"（"+causeReason(cause.Type)+"）")
			}
			if len(fields) == 8 {
				break
			}
		}
	}
	return errors.New("资源校验失败：请检查字段类型、必填项和不可修改字段 " + strings.Join(fields, "、"))
}

func causeReason(kind metav1.CauseType) string {
	switch kind {
	case metav1.CauseTypeFieldValueRequired:
		return "缺少必填字段"
	case metav1.CauseTypeFieldValueDuplicate:
		return "值重复"
	case metav1.CauseTypeFieldValueNotSupported:
		return "不支持的取值"
	case metav1.CauseTypeForbidden:
		return "字段不允许此修改"
	case metav1.CauseTypeTooLong:
		return "内容过长"
	case metav1.CauseTypeTooMany:
		return "条目过多"
	default:
		return "类型、取值或约束无效"
	}
}
