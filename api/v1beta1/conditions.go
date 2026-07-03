package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ready ConditionType = "Ready"
const passwordSynced ConditionType = "PasswordSynced"

type ConditionType string

type Condition struct {
	// Type indicates the scope of the custom resource status addressed by the condition.
	Type ConditionType `json:"type"`
	// True, False, or Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last time this Condition status changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// One word, camel-case reason for current status of the condition.
	Reason string `json:"reason,omitempty"`
	// Full text reason for current status of the condition.
	Message string `json:"message,omitempty"`
}

// Ready indicates that the last Create/Update operator on the CR was successful.
func Ready(lastConditions []Condition) Condition {
	time := lastTransitionTime(ready, corev1.ConditionTrue, lastConditions)
	return Condition{
		Type:               ready,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: time,
		Reason:             "SuccessfulCreateOrUpdate",
	}
}

// NotReady indicates that the last Create/Update operator on the CR failed.
func NotReady(msg string, lastConditions []Condition) Condition {
	time := lastTransitionTime(ready, corev1.ConditionFalse, lastConditions)
	return Condition{
		Type:               ready,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: time,
		Reason:             "FailedCreateOrUpdate",
		Message:            msg,
	}
}

// PasswordSynced indicates that the User's password was successfully synced to RabbitMQ.
func PasswordSynced(lastConditions []Condition) Condition {
	time := lastTransitionTime(passwordSynced, corev1.ConditionTrue, lastConditions)
	return Condition{
		Type:               passwordSynced,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: time,
		Reason:             "SuccessfulPasswordSync",
	}
}

// PasswordNotSynced indicates that the User's password failed to sync to RabbitMQ.
func PasswordNotSynced(msg string, lastConditions []Condition) Condition {
	time := lastTransitionTime(passwordSynced, corev1.ConditionFalse, lastConditions)
	return Condition{
		Type:               passwordSynced,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: time,
		Reason:             "FailedPasswordSync",
		Message:            msg,
	}
}

func lastTransitionTime(conditionType ConditionType, newStatus corev1.ConditionStatus, lastConditions []Condition) metav1.Time {
	for _, lastCondition := range lastConditions {
		if lastCondition.Type == conditionType && lastCondition.Status == newStatus {
			return lastCondition.LastTransitionTime
		}
	}
	return metav1.Now()
}

// upsertCondition replaces the condition in conditions whose Type matches newCondition's Type,
// or appends it if no such condition exists. Other condition types are left untouched.
func upsertCondition(conditions []Condition, newCondition Condition) []Condition {
	for i, c := range conditions {
		if c.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}
