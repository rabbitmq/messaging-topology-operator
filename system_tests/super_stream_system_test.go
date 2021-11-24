package system_tests

import (
	"context"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SuperStream", func() {
	var (
		namespace   = MustHaveEnv("NAMESPACE")
		ctx         = context.Background()
		superStream *topology.SuperStream
	)

	BeforeEach(func() {
		superStream = &topology.SuperStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "super-stream-test",
				Namespace: namespace,
			},
			Spec: topology.SuperStreamSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Name:       "super-stream-test",
				Partitions: 4,
				RoutingKeys: []string{
					"eu-west-1",
					"eu-west-2",
					"eu-west-3",
					"eu-west-4",
				},
			},
		}
	})

	It("creates and deletes a superStream successfully", func() {
		By("creating an exchange")
		Expect(k8sClient.Create(ctx, superStream, &client.CreateOptions{})).To(Succeed())
		var exchangeInfo *rabbithole.DetailedExchangeInfo
		Eventually(func() error {
			var err error
			exchangeInfo, err = rabbitClient.GetExchange("/", "super-stream-test")
			return err
		}, 10, 2).Should(BeNil())

		Expect(*exchangeInfo).To(MatchFields(IgnoreExtras, Fields{
			"Name":       Equal("super-stream-test"),
			"Vhost":      Equal("/"),
			"Type":       Equal("direct"),
			"AutoDelete": BeFalse(),
			"Durable":    BeTrue(),
		}))

		By("creating n queues")
		for _, routingKey := range superStream.Spec.RoutingKeys {
			var qInfo *rabbithole.DetailedQueueInfo
			Eventually(func() error {
				var err error
				qInfo, err = rabbitClient.GetQueue("/", fmt.Sprintf("super-stream-test.%s", routingKey))
				return err
			}, 10, 2).Should(BeNil())

			Expect(*qInfo).To(MatchFields(IgnoreExtras, Fields{
				"Name":       Equal(fmt.Sprintf("super-stream-test.%s", routingKey)),
				"Vhost":      Equal("/"),
				"AutoDelete": Equal(rabbithole.AutoDelete(false)),
				"Durable":    BeTrue(),
				"Type":       Equal("stream"),
			}))
		}

		By("creating n bindings")
		foundPartitionOrderStreams := make(map[int]rabbithole.BindingInfo)
		for _, routingKey := range superStream.Spec.RoutingKeys {
			var fetchedBinding rabbithole.BindingInfo
			Eventually(func() bool {
				var err error
				bindings, err := rabbitClient.ListBindingsIn("/")
				Expect(err).NotTo(HaveOccurred())
				for _, b := range bindings {
					if b.Source == "super-stream-test" && b.Destination == fmt.Sprintf("super-stream-test.%s", routingKey) {
						fetchedBinding = b
						return true
					}
				}
				return false
			}, 10, 2).Should(BeTrue(), "cannot find created binding")
			Expect(fetchedBinding).To(MatchFields(IgnoreExtras, Fields{
				"Vhost":           Equal("/"),
				"Source":          Equal("super-stream-test"),
				"Destination":     Equal(fmt.Sprintf("super-stream-test.%s", routingKey)),
				"DestinationType": Equal("queue"),
				"RoutingKey":      Equal(routingKey),
			}))
			Expect(fetchedBinding.Arguments).To(HaveKey("x-stream-partition-order"))
			partitionOrder := int(fetchedBinding.Arguments["x-stream-partition-order"].(float64))
			foundPartitionOrderStreams[partitionOrder] = fetchedBinding
		}
		for i := 0; i < superStream.Spec.Partitions; i++ {
			_, ok := foundPartitionOrderStreams[i]
			Expect(ok).To(
				BeTrue(),
				fmt.Sprintf("Expected to find a partition assigned to all indices, but %d was missing: %+v", i, foundPartitionOrderStreams),
			)
		}

		By("updating status condition 'Ready'")
		updatedSuperStream := topology.SuperStream{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStream.Name, Namespace: superStream.Namespace}, &updatedSuperStream)).To(Succeed())

		Expect(updatedSuperStream.Status.Conditions).To(HaveLen(1))
		readyCondition := updatedSuperStream.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedSuperStream.Status.ObservedGeneration).To(Equal(updatedSuperStream.GetGeneration()))

		By("deleting superStream")
		Expect(k8sClient.Delete(ctx, superStream)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetExchange("/", "super-stream-test")
			return err
		}, 10).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))

		By("deleting underlying resources")
		Eventually(func() error {
			var err error
			_, err = rabbitClient.GetExchange("/", "super-stream-test")
			return err
		}, 10, 2).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))

		for _, routingKey := range superStream.Spec.RoutingKeys {
			Eventually(func() error {
				var err error
				_, err = rabbitClient.GetQueue("/", fmt.Sprintf("super-stream-test.%s", routingKey))
				return err
			}, 10, 2).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Object Not Found"))

			Eventually(func() bool {
				bindings, err := rabbitClient.ListBindingsIn("/")
				Expect(err).NotTo(HaveOccurred())
				for _, b := range bindings {
					if b.Source == "super-stream-test" && b.Destination == fmt.Sprintf("super-stream-test.%s", routingKey) {
						return true
					}
				}
				return false
			}, 10, 2).Should(BeFalse(), "found the binding where we expected it to have been deleted")
		}
	})
})
