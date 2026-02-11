package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("policy webhook", func() {
	var (
		policy = rabbitmqcomv1beta1.OperatorPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: rabbitmqcomv1beta1.OperatorPolicySpec{
				Name:     "test",
				Vhost:    "/test",
				Pattern:  "a-pattern",
				ApplyTo:  "queues",
				Priority: 0,
				RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
		rootCtx                 = context.Background()
		operatorPolicyValidator OperatorPolicyCustomValidator
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := policy.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := operatorPolicyValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := policy.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := operatorPolicyValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on operator policy name", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.Name = "new-name"
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on vhost", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.Vhost = "new-vhost"
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.RabbitmqClusterReference = rabbitmqcomv1beta1.RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := rabbitmqcomv1beta1.OperatorPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: rabbitmqcomv1beta1.OperatorPolicySpec{
					Name:     "test",
					Vhost:    "/test",
					Pattern:  "a-pattern",
					ApplyTo:  "all",
					Priority: 0,
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newOperatorPolicy := connectionScr.DeepCopy()
			newOperatorPolicy.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &connectionScr, newOperatorPolicy)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("allows updates on operator policy.spec.pattern", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.Pattern = "new-pattern"
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(Succeed())
		})

		It("allows updates on operator policy.spec.applyTo", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.ApplyTo = "queues"
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(Succeed())
		})

		It("allows updates on operator policy.spec.priority", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.Priority = 1000
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(Succeed())
		})

		It("allows updates on operator policy.spec.definition", func() {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec.Definition = &runtime.RawExtension{Raw: []byte(`{"key":"new-definition-value"}`)}
			_, err := operatorPolicyValidator.ValidateUpdate(rootCtx, &policy, newPolicy)
			Expect(err).To(Succeed())
		})
	})
})
