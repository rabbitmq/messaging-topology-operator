package managedresource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SuperstreamExchange", func() {
	var (
		superStream     topologyv1alpha1.SuperStream
		builder         *managedresource.Builder
		exchangeBuilder *managedresource.SuperStreamExchangeBuilder
		exchange        *topology.Exchange
		scheme          *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(topology.AddToScheme(scheme)).To(Succeed())
		Expect(topologyv1alpha1.AddToScheme(scheme)).To(Succeed())
		superStream = topologyv1alpha1.SuperStream{}
		superStream.Namespace = "foo"
		superStream.Name = "foo"
		builder = &managedresource.Builder{
			ObjectOwner: &superStream,
			Scheme:      scheme,
		}
		exchangeBuilder = builder.SuperStreamExchange("vvv", testRabbitmqClusterReference)
		obj, _ := exchangeBuilder.Build()
		exchange = obj.(*topology.Exchange)
	})

	Context("Build", func() {
		It("generates an exchange object with the correct name", func() {
			Expect(exchange.Name).To(Equal("foo-exchange"))
		})

		It("generates an exchange object with the correct namespace", func() {
			Expect(exchange.Namespace).To(Equal(superStream.Namespace))
		})

		It("sets labels on the object to tie back to the original super stream", func() {
			Expect(exchange.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream", "foo"))
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			Expect(exchangeBuilder.Update(exchange)).To(Succeed())
		})
		It("sets owner reference", func() {
			Expect(exchange.OwnerReferences[0].Name).To(Equal(superStream.Name))
		})

		It("uses the name of the super stream as the name of the exchange", func() {
			Expect(exchange.Spec.Name).To(Equal(superStream.Name))
		})

		It("sets the vhost", func() {
			Expect(exchange.Spec.Vhost).To(Equal("vvv"))
		})

		It("generates a durable exchange", func() {
			Expect(exchange.Spec.Durable).To(BeTrue())
		})

		It("sets the expected RabbitmqClusterReference", func() {
			Expect(exchange.Spec.RabbitmqClusterReference.Name).To(Equal(testRabbitmqClusterReference.Name))
			Expect(exchange.Spec.RabbitmqClusterReference.Namespace).To(Equal(testRabbitmqClusterReference.Namespace))
		})
	})
})
