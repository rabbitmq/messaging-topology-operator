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
	var expectedQueueNames []string

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
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
				})

				It("creates the SuperStream and any underlying resources", func() {
					Expect(client.Create(ctx, &superStream)).To(Succeed())

					By("creating an exchange", func() {
						var exchange topology.Exchange
						EventuallyWithOffset(1, func() error {
								return client.Get(
									ctx,
									types.NamespacedName{Name: superStreamName + "-exchange", Namespace: "default"},
									&exchange,
								)
						}, 10*time.Second, 1*time.Second).Should(Succeed())
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
						expectedQueueNames = []string{}
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedQueueName := fmt.Sprintf("%s-partition-%s", superStreamName, strconv.Itoa(i))
							EventuallyWithOffset(1, func() error {
								return client.Get(
									ctx,
									types.NamespacedName{Name: expectedQueueName, Namespace: "default"},
									&partition,
								)
							}, 10*time.Second, 1*time.Second).Should(Succeed())

							expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

							Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Name":    Equal(fmt.Sprintf("%s-%s", superStreamName, strconv.Itoa(i))),
								"Type":    Equal("stream"),
								"Durable": BeTrue(),
								"RabbitmqClusterReference": MatchAllFields(Fields{
									"Name":      Equal("example-rabbit"),
									"Namespace": Equal("default"),
								}),
							}))
						}
					})

					By("setting the status of the super stream to list the partition queue names", func() {
						EventuallyWithOffset(1, func() []string {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: superStreamName, Namespace: "default"},
								&superStream,
							)

							return superStream.Status.Partitions
						}, 10*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
					})

					By("creating n bindings", func() {
						var binding topology.Binding
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedBindingName := fmt.Sprintf("%s-binding-%s", superStreamName, strconv.Itoa(i))
							EventuallyWithOffset(1, func() error {
								return client.Get(
									ctx,
									types.NamespacedName{Name: expectedBindingName, Namespace: "default"},
									&binding,
								)
							}, 10*time.Second, 1*time.Second).Should(Succeed())
							Expect(binding.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Source":          Equal(superStreamName),
								"DestinationType": Equal("queue"),
								"Destination":     Equal(fmt.Sprintf("%s-%s", superStreamName, strconv.Itoa(i))),
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
				})
			})

			When("the super stream is scaled", func() {
				JustBeforeEach(func() {
					Expect(client.Create(ctx, &superStream)).To(Succeed())
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
				When("the super stream is scaled out", func() {
					BeforeEach(func() {
						superStreamName = "scale-out-super-stream"
					})
					It("allows the number of partitions to be increased", func() {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: "default"},
							&superStream,
						)
						superStream.Spec.Partitions = 5
						Expect(client.Update(ctx, &superStream)).To(Succeed())

						By("creating n stream queue partitions", func() {
							var partition topology.Queue
							expectedQueueNames = []string{}
							for i := 0; i < superStream.Spec.Partitions; i++ {
								expectedQueueName := fmt.Sprintf("%s-partition-%s", superStreamName, strconv.Itoa(i))
								EventuallyWithOffset(1, func() error {
									return client.Get(
										ctx,
										types.NamespacedName{Name: expectedQueueName, Namespace: "default"},
										&partition,
									)
								}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
								expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

								Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
									"Name":    Equal(fmt.Sprintf("%s-%s", superStreamName, strconv.Itoa(i))),
									"Type":    Equal("stream"),
									"Durable": BeTrue(),
									"RabbitmqClusterReference": MatchAllFields(Fields{
										"Name":      Equal("example-rabbit"),
										"Namespace": Equal("default"),
									}),
								}))
							}
						})

						By("setting the status of the super stream to list the partition queue names", func() {
							EventuallyWithOffset(1, func() []string {
								_ = client.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: "default"},
									&superStream,
								)

								return superStream.Status.Partitions
							}, 10*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
						})

						By("creating n bindings", func() {
							var binding topology.Binding
							for i := 0; i < superStream.Spec.Partitions; i++ {
								expectedBindingName := fmt.Sprintf("%s-binding-%s", superStreamName, strconv.Itoa(i))
								EventuallyWithOffset(1, func() error {
									return client.Get(
										ctx,
										types.NamespacedName{Name: expectedBindingName, Namespace: "default"},
										&binding,
									)
								}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
								Expect(binding.Spec).To(MatchFields(IgnoreExtras, Fields{
									"Source":          Equal(superStreamName),
									"DestinationType": Equal("queue"),
									"Destination":     Equal(fmt.Sprintf("%s-%s", superStreamName, strconv.Itoa(i))),
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
					})
				})
				When("the super stream is scaled down", func() {
					var originalPartitionCount int
					BeforeEach(func() {
						superStreamName = "scale-down-super-stream"
					})
					It("refuses scaling down the partitions with a helpful warning", func() {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: "default"},
							&superStream,
						)
						originalPartitionCount = len(superStream.Status.Partitions)
						superStream.Spec.Partitions = 1
						Expect(client.Update(ctx, &superStream)).To(Succeed())

						By("setting the status condition 'Ready' to 'false' ", func() {
							EventuallyWithOffset(1, func() []topology.Condition {
								_ = client.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: "default"},
									&superStream,
								)

								return superStream.Status.Conditions
							}, 5*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(topology.ConditionType("Ready")),
								"Reason": Equal("FailedCreateOrUpdate"),
								"Status": Equal(corev1.ConditionFalse),
							})))
						})
						By("retaining the original stream queue partitions", func() {
							var partition topology.Queue
							expectedQueueNames = []string{}
							for i := 0; i < originalPartitionCount; i++ {
								expectedQueueName := fmt.Sprintf("%s-partition-%s", superStreamName, strconv.Itoa(i))
								Expect(client.Get(
									ctx,
									types.NamespacedName{Name: expectedQueueName, Namespace: "default"},
									&partition,
								)).To(Succeed())
								expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

								Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
									"Name":    Equal(fmt.Sprintf("%s-%s", superStreamName, strconv.Itoa(i))),
									"Type":    Equal("stream"),
									"Durable": BeTrue(),
									"RabbitmqClusterReference": MatchAllFields(Fields{
										"Name":      Equal("example-rabbit"),
										"Namespace": Equal("default"),
									}),
								}))
							}
						})

						By("setting the status of the super stream to list the partition queue names", func() {
							ConsistentlyWithOffset(1, func() []string {
								_ = client.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: "default"},
									&superStream,
								)

								return superStream.Status.Partitions
							}, 5*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
						})

						By("retaining the original bindings", func() {
							var binding topology.Binding
							for i := 0; i < originalPartitionCount; i++ {
								expectedBindingName := fmt.Sprintf("%s-binding-%s", superStreamName, strconv.Itoa(i))
								EventuallyWithOffset(1, func() error {
									return client.Get(
										ctx,
										types.NamespacedName{Name: expectedBindingName, Namespace: "default"},
										&binding,
									)
								}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
								Expect(binding.Spec).To(MatchFields(IgnoreExtras, Fields{
									"Source":          Equal(superStreamName),
									"DestinationType": Equal("queue"),
									"Destination":     Equal(fmt.Sprintf("%s-%s", superStreamName, strconv.Itoa(i))),
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

			When("routing keys are specifically set", func() {
				BeforeEach(func() {
					superStreamName = "specific-keys-stream"
				})

				It("creates the SuperStream and any underlying resources", func() {
					superStream.Spec.RoutingKeys = []string{"abc", "bcd", "cde"}
					superStream.Spec.Partitions = 3
					Expect(client.Create(ctx, &superStream)).To(Succeed())

					By("setting the status condition 'Ready' to 'true' ", func() {
						EventuallyWithOffset(1, func() []topology.Condition {
							var fetchedSuperStream topology.SuperStream
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: superStreamName, Namespace: "default"},
								&fetchedSuperStream,
							)

							return fetchedSuperStream.Status.Conditions
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
						expectedQueueNames = []string{}
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedQueueName := fmt.Sprintf("%s-partition-%s", superStreamName, superStream.Spec.RoutingKeys[i])
							err := client.Get(
								ctx,
								types.NamespacedName{Name: expectedQueueName, Namespace: "default"},
								&partition,
							)
							Expect(err).NotTo(HaveOccurred())

							expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

							Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Name":    Equal(fmt.Sprintf("%s-%s", superStreamName, superStream.Spec.RoutingKeys[i])),
								"Type":    Equal("stream"),
								"Durable": BeTrue(),
								"RabbitmqClusterReference": MatchAllFields(Fields{
									"Name":      Equal("example-rabbit"),
									"Namespace": Equal("default"),
								}),
							}))
						}
					})

					By("setting the status of the super stream to list the partition queue names", func() {
						EventuallyWithOffset(1, func() []string {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: superStreamName, Namespace: "default"},
								&superStream,
							)

							return superStream.Status.Partitions
						}, 10*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
					})

					By("creating n bindings", func() {
						var binding topology.Binding
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedBindingName := fmt.Sprintf("%s-binding-%s", superStreamName, superStream.Spec.RoutingKeys[i])
							err := client.Get(
								ctx,
								types.NamespacedName{Name: expectedBindingName, Namespace: "default"},
								&binding,
							)
							Expect(err).NotTo(HaveOccurred())
							Expect(binding.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Source":          Equal(superStreamName),
								"DestinationType": Equal("queue"),
								"Destination":     Equal(fmt.Sprintf("%s-%s", superStreamName, superStream.Spec.RoutingKeys[i])),
								"Arguments": PointTo(MatchFields(IgnoreExtras, Fields{
									"Raw": Equal([]byte(fmt.Sprintf(`{"x-stream-partition-order":%d}`, i))),
								})),
								"RoutingKey": Equal(superStream.Spec.RoutingKeys[i]),
								"RabbitmqClusterReference": MatchAllFields(Fields{
									"Name":      Equal("example-rabbit"),
									"Namespace": Equal("default"),
								}),
							}))
						}
					})
				})
			})
			When("the number of routing keys does not match the partition count", func() {
				BeforeEach(func() {
					superStreamName = "mismatch"
				})

				It("creates the SuperStream and any underlying resources", func() {
					superStream.Spec.RoutingKeys = []string{"abc", "bcd", "cde"}
					superStream.Spec.Partitions = 2
					Expect(client.Create(ctx, &superStream)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						var fetchedSuperStream topology.SuperStream
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: "default"},
							&fetchedSuperStream,
						)

						return fetchedSuperStream.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Message": Equal("SuperStream mismatch failed to reconcile"),
						"Status":  Equal(corev1.ConditionFalse),
					})))
				})
			})
		})
	})
})
