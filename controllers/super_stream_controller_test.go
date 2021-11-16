package controllers_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"strconv"
	"time"
)

var _ = Describe("super-stream-controller", func() {

	var superStream topology.SuperStream
	var superStreamName string

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
			superStream = topology.SuperStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      superStreamName,
					Namespace: "default",
				},
				Spec: topology.SuperStreamSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
					Partitions: 3,
				},
			}
		})

		Context("creation", func() {
			When("success", func() {
				BeforeEach(func() {
					superStreamName = "basic-super-stream"
					fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
				})

				It("creates the SuperStream and any underlying resources", func() {
					Expect(client.Create(ctx, &superStream)).To(Succeed())

					By("setting the status condition 'Ready' to 'true' ", func() {
						EventuallyWithOffset(1, func() []topology.Condition {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: superStreamName, Namespace: "default"},
								&superStream,
							)

							return superStream.Status.Conditions
						}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					})
					By("creating an exchange", func() {
						var exchange topology.Exchange
						err := client.Get(
							ctx,
							types.NamespacedName{Name: superStreamName + "-exchange", Namespace: "default"},
							&exchange,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(exchange.Spec).To(MatchFields(IgnoreExtras, Fields{
							"Name":    Equal(superStreamName),
							"Type":    Equal("direct"),
							"Durable": BeTrue(),
							"RabbitmqClusterReference": MatchAllFields(Fields{
								"Name":      Equal("example-rabbit"),
								"Namespace": Equal("default"),
							}),
						}))

					})
					By("creating n stream queue partitions", func() {
						var partition topology.Queue
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedQueueName := fmt.Sprintf("%s-partition-%s", superStreamName, strconv.Itoa(i))
							err := client.Get(
								ctx,
								types.NamespacedName{Name: expectedQueueName, Namespace: "default"},
								&partition,
							)
							Expect(err).NotTo(HaveOccurred())
							Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Name":    Equal(fmt.Sprintf("%s.%s", superStreamName, strconv.Itoa(i))),
								"Type":    Equal("stream"),
								"Durable": BeTrue(),
								"RabbitmqClusterReference": MatchAllFields(Fields{
									"Name":      Equal("example-rabbit"),
									"Namespace": Equal("default"),
								}),
							}))
						}
					})

					By("creating n bindings", func() {
						var binding topology.Binding
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedBindingName := fmt.Sprintf("%s-binding-%s", superStreamName, strconv.Itoa(i))
							err := client.Get(
								ctx,
								types.NamespacedName{Name: expectedBindingName, Namespace: "default"},
								&binding,
							)
							Expect(err).NotTo(HaveOccurred())
							Expect(binding.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Source":          Equal(superStreamName),
								"DestinationType": Equal("queue"),
								"Destination":     Equal(fmt.Sprintf("%s.%s", superStreamName, strconv.Itoa(i))),
								"Arguments": PointTo(MatchFields(IgnoreExtras, Fields{
									"Raw": Equal([]byte(fmt.Sprintf(`{"x-stream-partition-order":%d}`, i))),
								})),
								"RoutingKey": Equal(strconv.Itoa(i)),
								"RabbitmqClusterReference": MatchAllFields(Fields{
									"Name":      Equal("example-rabbit"),
									"Namespace": Equal("default"),
								}),
							}))
						}

					})
				})
			})
		})
	})
})
