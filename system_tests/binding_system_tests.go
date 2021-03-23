package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

var _ = Describe("Binding", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		binding   *topologyv1alpha1.Binding
		queue     *topologyv1alpha1.Queue
		exchange  *topologyv1alpha1.Exchange
	)

	BeforeEach(func() {
		exchange = &topologyv1alpha1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exchange",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.ExchangeSpec{
				Name: "test-exchange",
				RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, exchange, &client.CreateOptions{})).To(Succeed())
		queue = &topologyv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-queue",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.QueueSpec{
				Name: "test-queue",
				RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, queue, &client.CreateOptions{})).To(Succeed())
		Eventually(func() error {
			var err error
			_, err = rabbitClient.GetQueue(queue.Spec.Vhost, queue.Name)
			return err
		}, 10, 2).Should(BeNil()) // wait for queue to be available; or else binding will fail to create

		binding = &topologyv1alpha1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.BindingSpec{
				RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
				Source:          "test-exchange",
				Destination:     "test-queue",
				DestinationType: "queue",
				RoutingKey:      "test-key",
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"extra-argument": "test"}`),
				},
			},
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, binding)).To(Succeed())
		Expect(k8sClient.Delete(ctx, queue)).To(Succeed())
		Expect(k8sClient.Delete(ctx, exchange)).To(Succeed())
	})

	It("declares a binding successfully", func() {
		Expect(k8sClient.Create(ctx, binding, &client.CreateOptions{})).To(Succeed())
		var fetchedBinding rabbithole.BindingInfo
		Eventually(func() bool {
			var err error
			bindings, err := rabbitClient.ListBindingsIn(binding.Spec.Vhost)
			Expect(err).NotTo(HaveOccurred())
			for _, b := range bindings {
				if b.Source == binding.Spec.Source {
					fetchedBinding = b
					return true
				}
			}
			return false
		}, 10, 2).Should(BeTrue(), "cannot find created binding")
		Expect(fetchedBinding).To(MatchFields(IgnoreExtras, Fields{
			"Vhost":           Equal(binding.Spec.Vhost),
			"Source":          Equal(binding.Spec.Source),
			"Destination":     Equal(binding.Spec.Destination),
			"DestinationType": Equal(binding.Spec.DestinationType),
			"RoutingKey":      Equal(binding.Spec.RoutingKey),
		}))
		Expect(fetchedBinding.Arguments).To(HaveKeyWithValue("extra-argument", "test"))

		By("updating status condition 'Ready'")
		updatedBinding := topologyv1alpha1.Binding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &updatedBinding)).To(Succeed())

		Expect(updatedBinding.Status.Conditions).To(HaveLen(1))
		readyCondition := updatedBinding.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedBinding.Status.ObservedGeneration).To(Equal(updatedBinding.GetGeneration()))

		By("not allowing updates on binding.spec")
		updateBinding := topologyv1alpha1.Binding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &updateBinding)).To(Succeed())
		updatedBinding.Spec.RoutingKey = "new-key"
		Expect(k8sClient.Update(ctx, &updatedBinding).Error()).To(ContainSubstring("invalid: spec.routingKey: Invalid value: \"new-key\": routingKey cannot be updated"))
	})
})
