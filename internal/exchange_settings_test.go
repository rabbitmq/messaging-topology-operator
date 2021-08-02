package internal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("GenerateExchangeSettings", func() {
	var e *topology.Exchange

	BeforeEach(func() {
		e = &topology.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name: "exchange",
			},
			Spec: topology.ExchangeSpec{
				Type:       "fanout",
				Durable:    true,
				AutoDelete: true,
			},
		}
	})

	It("sets the type according to exchange.spec", func() {
		settings, err := internal.GenerateExchangeSettings(e)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Type).To(Equal("fanout"))
	})

	It("sets AutoDelete according to exchange.spec", func() {
		settings, err := internal.GenerateExchangeSettings(e)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.AutoDelete).To(BeTrue())
	})

	It("sets Durable according to exchange.spec", func() {
		settings, err := internal.GenerateExchangeSettings(e)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Durable).To(BeTrue())
	})

	When("exchange arguments are provided", func() {
		It("generates the correct exchange arguments", func() {
			e.Spec.Arguments = &runtime.RawExtension{
				Raw: []byte(`{"alternate-exchange": "alt-exchange"}`),
			}
			settings, err := internal.GenerateExchangeSettings(e)
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Arguments).To(HaveLen(1))
			Expect(settings.Arguments).To(HaveKeyWithValue("alternate-exchange", "alt-exchange"))
		})
	})

})
