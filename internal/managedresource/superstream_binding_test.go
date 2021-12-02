package managedresource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SuperstreamBinding", func() {
	var (
		superStream    topology.SuperStream
		builder        *managedresource.Builder
		bindingBuilder *managedresource.SuperStreamBindingBuilder
		binding        *topology.Binding
		scheme         *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(topology.AddToScheme(scheme)).To(Succeed())
		superStream = topology.SuperStream{}
		superStream.Namespace = "foo"
		superStream.Name = "foo"
		builder = &managedresource.Builder{
			ObjectOwner: &superStream,
			Scheme:      scheme,
		}
		bindingBuilder = builder.SuperStreamBinding(678, "emea", "vvv", testRabbitmqClusterReference)
		obj, _ := bindingBuilder.Build()
		binding = obj.(*topology.Binding)
	})

	Context("Build", func() {
		It("generates a binding object with the correct name", func() {
			Expect(binding.Name).To(Equal("foo-binding-emea"))
		})

		It("generates a binding object with the correct namespace", func() {
			Expect(binding.Namespace).To(Equal(superStream.Namespace))
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			Expect(bindingBuilder.Update(binding)).To(Succeed())
		})
		It("sets owner reference", func() {
			Expect(binding.OwnerReferences[0].Name).To(Equal(superStream.Name))
		})

		It("sets the Source to the name of the exchange", func() {
			Expect(binding.Spec.Source).To(Equal("foo"))
		})

		It("sets the DestinationType to queue", func() {
			Expect(binding.Spec.DestinationType).To(Equal("queue"))
		})

		It("sets the Destination to the partition queue", func() {
			Expect(binding.Spec.Destination).To(Equal("foo-emea"))
		})

		It("sets the stream partition args", func() {
			Expect(binding.Spec.Arguments.Raw).To(Equal([]byte(`{"x-stream-partition-order": 678}`)))
		})

		It("sets the routing key", func() {
			Expect(binding.Spec.RoutingKey).To(Equal("emea"))
		})

		It("sets the vhost", func() {
			Expect(binding.Spec.Vhost).To(Equal("vvv"))
		})

		It("sets the expected RabbitmqClusterReference", func() {
			Expect(binding.Spec.RabbitmqClusterReference.Name).To(Equal(testRabbitmqClusterReference.Name))
			Expect(binding.Spec.RabbitmqClusterReference.Namespace).To(Equal(testRabbitmqClusterReference.Namespace))
		})
	})
})
