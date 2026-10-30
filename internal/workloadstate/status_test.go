package workloadstate

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"strings"
	"testing"
)

func TestReportedWorkloadStates(t *testing.T) {
	tests := []struct{ name, api, kind, body, want, detail string }{
		{"partial ready", "apps/v1", "Deployment", `"spec":{"replicas":3},"status":{"replicas":3,"readyReplicas":1}`, Ready, "1 / 3"},
		{"scaled to zero", "apps/v1", "StatefulSet", `"spec":{"replicas":0},"status":{"replicas":0}`, Stopped, "0 / 0"},
		{"scaling down", "apps/v1", "Deployment", `"spec":{"replicas":0},"status":{"replicas":2}`, Processing, "正在停止"},
		{"new controller", "apps/v1", "ReplicaSet", `"spec":{"replicas":2}`, Processing, "0 / 2"},
		{"failure beats ready", "apps/v1", "Deployment", `"spec":{"replicas":3},"status":{"readyReplicas":1,"conditions":[{"type":"Progressing","status":"False","reason":"ProgressDeadlineExceeded"}]}`, Error, "ProgressDeadlineExceeded"},
		{"replica failure", "apps/v1", "ReplicaSet", `"status":{"conditions":[{"type":"ReplicaFailure","status":"True","reason":"FailedCreate"}]}`, Error, "FailedCreate"},
		{"ds has no eligible nodes", "apps/v1", "DaemonSet", `"status":{"desiredNumberScheduled":0,"currentNumberScheduled":0}`, Stopped, "0 / 0"},
		{"ds not observed", "apps/v1", "DaemonSet", `"status":{}`, Processing, "等待控制器"},
		{"job succeeds", "batch/v1", "Job", `"status":{"succeeded":1,"conditions":[{"type":"Complete","status":"True"}]}`, Completed, "成功 1"},
		{"job failed", "batch/v1", "Job", `"status":{"conditions":[{"type":"Failed","status":"True","reason":"BackoffLimitExceeded"}]}`, Error, "BackoffLimitExceeded"},
		{"job retries", "batch/v1", "Job", `"status":{"failed":1,"active":1}`, Processing, "失败 1"},
		{"job ready", "batch/v1", "Job", `"status":{"ready":1,"active":1}`, Ready, "运行 1"},
		{"job suspended", "batch/v1", "Job", `"spec":{"suspend":true}`, Stopped, "已暂停"},
		{"cron suspended", "batch/v1", "CronJob", `"spec":{"suspend":true,"schedule":"* * * * *"}`, Stopped, "已暂停"},
		{"cron completed", "batch/v1", "CronJob", `"status":{"lastScheduleTime":"2026-10-28T00:00:00Z","lastSuccessfulTime":"2026-10-28T00:01:00Z"}`, Completed, "最近任务已完成"},
		{"cron pending", "batch/v1", "CronJob", `"status":{"lastScheduleTime":"2026-10-28T00:02:00Z","lastSuccessfulTime":"2026-10-28T00:01:00Z"}`, Processing, "等待调度"},
		{"running pod", "v1", "Pod", `"status":{"phase":"Running","containerStatuses":[{"ready":true}]}`, Ready, "Running"},
		{"pod readiness gate", "v1", "Pod", `"status":{"phase":"Running","containerStatuses":[{"ready":true}],"conditions":[{"type":"Ready","status":"False"}]}`, Processing, "Running"},
		{"completed job pod", "v1", "Pod", `"status":{"phase":"Succeeded"}`, Completed, "运行完成"},
		{"container creating", "v1", "Pod", `"status":{"phase":"Pending","containerStatuses":[{"state":{"waiting":{"reason":"ContainerCreating"}}}]}`, Processing, "Pending"},
		{"crash loop", "v1", "Pod", `"status":{"phase":"Running","containerStatuses":[{"state":{"waiting":{"reason":"CrashLoopBackOff"}}}]}`, Error, "CrashLoopBackOff"},
		{"init image error", "v1", "Pod", `"status":{"phase":"Pending","initContainerStatuses":[{"state":{"waiting":{"reason":"ImagePullBackOff"}}}]}`, Error, "ImagePullBackOff"},
		{"nonzero exit", "v1", "Pod", `"status":{"phase":"Running","containerStatuses":[{"state":{"terminated":{"exitCode":1,"reason":"Error"}}}]}`, Error, "Error"},
		{"unschedulable", "v1", "Pod", `"status":{"conditions":[{"type":"PodScheduled","status":"False","reason":"Unschedulable"}]}`, Error, "Unschedulable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]interface{}{}
			if err := json.Unmarshal([]byte(`{"apiVersion":"`+tt.api+`","kind":"`+tt.kind+`",`+tt.body+`}`), &raw); err != nil {
				t.Fatal(err)
			}
			got, detail, ok := Classify(unstructured.Unstructured{Object: raw})
			if !ok || got != tt.want || !strings.Contains(detail, tt.detail) {
				t.Fatalf("state=%s detail=%s supported=%v", got, detail, ok)
			}
		})
	}
}
func TestUnobservedFailureAndCustomKindAreNotMisclassified(t *testing.T) {
	object := unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]interface{}{"generation": int64(2)}, "status": map[string]interface{}{"observedGeneration": int64(1), "conditions": []interface{}{map[string]interface{}{"type": "ReplicaFailure", "status": "True"}}}}}
	if got, _, _ := Classify(object); got != Processing {
		t.Fatalf("stale failure=%s", got)
	}
	object.SetAPIVersion("demo.example.com/v1")
	if _, _, ok := Classify(object); ok {
		t.Fatal("custom resource sharing a kind was classified as built-in")
	}
}
