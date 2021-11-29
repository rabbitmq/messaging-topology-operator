package managedresource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SuperstreamExchange", func() {
	var (
		builder                     *managedresource.Builder
		compositeConsumer           *topology.CompositeConsumer
		compositeConsumerPodBuilder *managedresource.CompositeConsumerPodBuilder
		pod                         *corev1.Pod
		podSpec                     corev1.PodSpec
		scheme                      *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(topology.AddToScheme(scheme)).To(Succeed())
		compositeConsumer = &topology.CompositeConsumer{}
		compositeConsumer.Name = "parent-set"
		compositeConsumer.Namespace = "parent-namespace"
		compositeConsumer.Spec.SuperStreamReference = topology.SuperStreamReference{
			Name:      "super-stream-1",
			Namespace: "parent-namespace",
		}

		podSpec = corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "example-container",
					Image: "example-image",
				},
			},
		}

		builder = &managedresource.Builder{
			ObjectOwner: compositeConsumer,
			Scheme:      scheme,
		}
		compositeConsumerPodBuilder = builder.CompositeConsumerPod(podSpec, "super-stream-1", "sample-partition")
		obj, _ := compositeConsumerPodBuilder.Build()
		pod = obj.(*corev1.Pod)
	})

	Context("Build", func() {
		It("generates an exchange object with the correct name", func() {
			Expect(pod.Name).To(Equal("parent-set-sample-partition"))
		})

		It("generates an pod object with the correct namespace", func() {
			Expect(pod.Namespace).To(Equal(compositeConsumer.Namespace))
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			Expect(compositeConsumerPodBuilder.Update(pod)).To(Succeed())
		})
		It("sets owner reference", func() {
			Expect(pod.OwnerReferences[0].Name).To(Equal(compositeConsumer.Name))
		})
		It("sets expected labels on the Pod", func() {
			Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream", "super-stream-1"))
			Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream-partition", "sample-partition"))
		})

	})
})
