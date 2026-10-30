package kube

import (
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidationCausesShowFieldsAndReasonsWithoutRejectedValues(t *testing.T) {
	err := &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonInvalid, Details: &metav1.StatusDetails{
		Causes: []metav1.StatusCause{{Type: metav1.CauseTypeFieldValueRequired, Field: "spec.containers[0].image", Message: "token=private-credential"}},
	}}}
	message := requestError(err).Error()
	if !strings.Contains(message, "spec.containers[0].image（缺少必填字段）") || strings.Contains(message, "private-credential") {
		t.Fatalf("unsafe or unhelpful diagnostic: %s", message)
	}
}
