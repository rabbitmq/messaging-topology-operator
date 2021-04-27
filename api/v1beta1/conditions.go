package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ready ConditionType = "Ready"

type ConditionType string

type Condition struct {
	// Type indicates the scope of RabbitmqCluster status addressed by the condition.
	Type ConditionType `json:"type"`
	// True, False, or Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last time this Condition type changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// One word, camel-case reason for current status of the condition.
	Reason string `json:"reason,omitempty"`
	// Full text reason for current status of the condition.
	Message string `json:"message,omitempty"`
}

// Ready indicates that the last Create/Update operator on the CR was successful.
func Ready() Condition {
	return Condition{
		Type:               ready,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "SuccessfulCreateOrUpdate",
	}
}

// NotReady indicates that the last Create/Update operator on the CR failed.
func NotReady(msg string) Condition {
	return Condition{
		Type:               ready,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "FailedCreateOrUpdate",
		Message:            msg,
	}
}
