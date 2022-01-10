package managedresource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SuperstreamPartition", func() {
	var (
		superStream      topologyv1alpha1.SuperStream
		builder          *managedresource.Builder
		partitionBuilder *managedresource.SuperStreamPartitionBuilder
		partition        *topology.Queue
		scheme           *runtime.Scheme
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
		partitionBuilder = builder.SuperStreamPartition(345, "emea", "vvv", testRabbitmqClusterReference)
		obj, _ := partitionBuilder.Build()
		partition = obj.(*topology.Queue)
	})

	Context("Build", func() {
		It("generates an partition object with the correct name", func() {
			Expect(partition.Name).To(Equal("foo-partition-345"))
		})

		It("generates an partition object with the correct namespace", func() {
			Expect(partition.Namespace).To(Equal(superStream.Namespace))
		})

		It("sets labels on the object to tie back to the original super stream", func() {
			Expect(partition.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream", "foo"))
			Expect(partition.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream-routing-key", "emea"))
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			Expect(partitionBuilder.Update(partition)).To(Succeed())
		})
		It("sets owner reference", func() {
			Expect(partition.OwnerReferences[0].Name).To(Equal(superStream.Name))
		})

		It("sets the queue to be durable", func() {
			Expect(partition.Spec.Durable).To(BeTrue())
		})

		It("sets the queue type to be stream", func() {
			Expect(partition.Spec.Type).To(Equal("stream"))
		})

		It("sets the name of the partition queue", func() {
			Expect(partition.Spec.Name).To(Equal("foo-emea"))
		})

		It("sets the vhost", func() {
			Expect(partition.Spec.Vhost).To(Equal("vvv"))
		})

		It("sets the expected RabbitmqClusterReference", func() {
			Expect(partition.Spec.RabbitmqClusterReference.Name).To(Equal(testRabbitmqClusterReference.Name))
			Expect(partition.Spec.RabbitmqClusterReference.Namespace).To(Equal(testRabbitmqClusterReference.Namespace))
		})
	})
})
