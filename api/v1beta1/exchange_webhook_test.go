package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("exchange webhook", func() {

	var (
		exchange = Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-exchange",
			},
			Spec: ExchangeSpec{
				Name:       "test",
				Vhost:      "/test",
				Type:       "fanout",
				Durable:    false,
				AutoDelete: true,
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		rootCtx           = context.Background()
		exchangeValidator ExchangeValidator
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := exchange.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := exchangeValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := exchange.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := exchangeValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on exchange name", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Name = "new-name"
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on vhost", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Vhost = "/a-new-vhost"
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-exchange",
				},
				Spec: ExchangeSpec{
					Name:  "test",
					Vhost: "/test",
					Type:  "fanout",
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newExchange := connectionScr.DeepCopy()
			newExchange.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &connectionScr, newExchange)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on exchange type", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Type = "direct"
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(MatchError(ContainSubstring("exchange type cannot be updated")))
		})

		It("does not allow updates on durable", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Durable = true
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(MatchError(ContainSubstring("durable cannot be updated")))
		})

		It("does not allow updates on autoDelete", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.AutoDelete = false
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(MatchError(ContainSubstring("autoDelete cannot be updated")))
		})

		It("allows updates on arguments", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Arguments = &runtime.RawExtension{Raw: []byte(`{"new":"new-value"}`)}
			_, err := exchangeValidator.ValidateUpdate(rootCtx, &exchange, newExchange)
			Expect(err).To(Succeed())
		})
	})
})
