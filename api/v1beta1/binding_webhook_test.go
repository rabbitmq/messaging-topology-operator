package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Binding webhook", func() {

	var (
		oldBinding = Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Vhost:           "/test",
				Source:          "test",
				Destination:     "test",
				DestinationType: "queue",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		rootCtx          = context.Background()
		bindingValidator BindingValidator
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := oldBinding.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}

			_, err := bindingValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).
				To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := oldBinding.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil

			_, err := bindingValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on vhost", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Vhost = "/new-vhost"

			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("updates on vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on RabbitmqClusterReference name", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("updates on vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference connectionSecret", func() {
			connectionScr := Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "connect-test-queue",
				},
				Spec: BindingSpec{
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newConnSrc := connectionScr.DeepCopy()
			newConnSrc.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			_, err := bindingValidator.ValidateUpdate(rootCtx, &connectionScr, newConnSrc)
			Expect(err).To(MatchError(ContainSubstring("updates on vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on source", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Source = "updated-source"
			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("source cannot be updated")))
		})

		It("does not allow updates on destination", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Destination = "updated-des"
			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("destination cannot be updated")))
		})

		It("does not allow updates on destination type", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.DestinationType = "exchange"
			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("destinationType cannot be updated")))
		})

		It("does not allow updates on routing key", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.RoutingKey = "not-allowed"
			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("routingKey cannot be updated")))
		})

		It("does not allow updates on binding arguments", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Arguments = &runtime.RawExtension{Raw: []byte(`{"new":"new-value"}`)}
			_, err := bindingValidator.ValidateUpdate(rootCtx, &oldBinding, newBinding)
			Expect(err).To(MatchError(ContainSubstring("arguments cannot be updated")))
		})
	})
})
