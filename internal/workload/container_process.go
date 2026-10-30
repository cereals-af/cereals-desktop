package workload

import (
	"errors"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func editContainerProcess(raw map[string]interface{}, old, next Container) error {
	if len(next.Command) > 0 && strings.TrimSpace(next.Command[0]) == "" {
		return errors.New("command 的第一项需要填写可执行程序；恢复镜像默认命令请移除全部命令项")
	}
	for _, field := range []struct {
		name      string
		old, next []string
	}{
		{"command", old.Command, next.Command},
		{"args", old.Args, next.Args},
	} {
		for _, value := range field.next {
			if strings.ContainsRune(value, '\x00') {
				return errors.New("command 和 args 不能包含 NUL 字符")
			}
		}
		if slices.Equal(field.old, field.next) {
			continue
		}
		if len(field.next) == 0 {
			delete(raw, field.name)
		} else if err := unstructured.SetNestedStringSlice(raw, field.next, field.name); err != nil {
			return err
		}
	}
	return nil
}
