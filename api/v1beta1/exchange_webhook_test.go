package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("exchange webhook", func() {

	var exchange = Exchange{
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

	It("does not allow updates on exchange name", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.Name = "new-name"
		Expect(apierrors.IsForbidden(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
	})

	It("does not allow updates on vhost", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.Vhost = "/a-new-vhost"
		Expect(apierrors.IsForbidden(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
	})

	It("does not allow updates on exchange type", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.Type = "direct"
		Expect(apierrors.IsInvalid(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
	})

	It("does not allow updates on durable", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.Durable = true
		Expect(apierrors.IsInvalid(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
	})

	It("does not allow updates on autoDelete", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.AutoDelete = false
		Expect(apierrors.IsInvalid(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
	})

	It("allows updates on arguments", func() {
		newExchange := exchange.DeepCopy()
		newExchange.Spec.Arguments = &runtime.RawExtension{Raw: []byte(`{"new":"new-value"}`)}
		Expect(newExchange.ValidateUpdate(&exchange)).To(Succeed())
	})
})
