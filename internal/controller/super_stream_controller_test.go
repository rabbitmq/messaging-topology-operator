package controller_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("super-stream-controller", func() {

	var (
		superStream        topologyv1alpha1.SuperStream
		superStreamName    string
		expectedQueueNames []string
		superStreamMgr     ctrl.Manager
		managerCtx         context.Context
		managerCancel      context.CancelFunc
		k8sClient          runtimeClient.Client
	)

	When("validating RabbitMQ Client failures", func() {
		BeforeEach(func() {
			var err error
			superStreamMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
				Metrics: server.Options{
					BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
				},
				Cache: cache.Options{
					DefaultNamespaces: map[string]cache.Config{superStreamNamespace: {}},
				},
				Logger: GinkgoLogr,
				Controller: config.Controller{
					SkipNameValidation: &skipNameValidation,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			managerCtx, managerCancel = context.WithCancel(context.Background())
			go func(ctx context.Context) {
				defer GinkgoRecover()
				Expect(superStreamMgr.Start(ctx)).To(Succeed())
			}(managerCtx)

			k8sClient = superStreamMgr.GetClient()

			Expect((&controller.SuperStreamReconciler{
				Log:                   GinkgoLogr,
				Client:                superStreamMgr.GetClient(),
				Scheme:                superStreamMgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
			}).SetupWithManager(superStreamMgr)).To(Succeed())
		})

		AfterEach(func() {
			managerCancel()
			// Sad workaround to avoid controllers racing for the reconciliation of other's
			// test cases. Without this wait, the last run test consistently fails because
			// the previous cancelled manager is just in time to reconcile the Queue of the
			// new/last test, and use the wrong/unexpected arguments in the queue declare call
			//
			// Eventual consistency is nice when you have good means of awaiting. That's not the
			// case with testenv and kubernetes controller.
			<-time.After(time.Second)
		})

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
			fakeRabbitMQClient.DeleteExchangeReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			fakeRabbitMQClient.DeleteQueueReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			fakeRabbitMQClient.DeleteBindingReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			superStream = topologyv1alpha1.SuperStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      superStreamName,
					Namespace: superStreamNamespace,
				},
				Spec: topologyv1alpha1.SuperStreamSpec{
					RabbitmqClusterReference: topologyv1beta1.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
					Partitions: 3,
				},
			}
		})

		Context("creation", func() {
			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &superStream)).To(Succeed())
			})

			When("an underlying resource is deleted", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(ctx, &superStream)).To(Succeed())
					EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
							&superStream,
						)
						return superStream.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
				})

				When("a binding is deleted", func() {
					BeforeEach(func() {
						superStreamName = "delete-binding"
					})

					It("recreates the missing object", func() {
						var binding topologyv1beta1.Binding
						expectedBindingName := fmt.Sprintf("%s-binding-2", superStreamName)
						Expect(k8sClient.Get(
							ctx,
							types.NamespacedName{Name: expectedBindingName, Namespace: superStreamNamespace},
							&binding,
						)).To(Succeed())
						initialUID := binding.GetUID()
						Expect(k8sClient.Delete(ctx, &binding, runtimeClient.GracePeriodSeconds(0))).To(Succeed())
						Eventually(komega.Get(&binding)).Within(time.Second * 10).WithPolling(time.Second).ShouldNot(Succeed())

						By("setting the status condition 'Ready' to 'true' ", func() {
							EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Conditions
							}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
								"Reason": Equal("SuccessfulCreateOrUpdate"),
								"Status": Equal(corev1.ConditionTrue),
							})))
						})

						By("recreating the binding", func() {
							EventuallyWithOffset(1, func() bool {
								err := k8sClient.Get(
									ctx,
									types.NamespacedName{Name: expectedBindingName, Namespace: superStreamNamespace},
									&binding,
								)
								if err != nil {
									return false
								}
								return binding.GetUID() != initialUID
							}, 10*time.Second, 1*time.Second).Should(BeTrue())
						})
					})
				})

				When("a queue is deleted", func() {
					BeforeEach(func() {
						superStreamName = "delete-queue"
					})

					It("recreates the missing object", func() {
						var queue topologyv1beta1.Queue
						expectedQueueName := fmt.Sprintf("%s-partition-1", superStreamName)
						Expect(k8sClient.Get(
							ctx,
							types.NamespacedName{Name: expectedQueueName, Namespace: superStreamNamespace},
							&queue,
						)).To(Succeed())
						initialUID := queue.GetUID()
						Expect(k8sClient.Delete(ctx, &queue, runtimeClient.GracePeriodSeconds(0))).To(Succeed())
						Eventually(komega.Get(&queue)).Within(time.Second * 10).WithPolling(time.Second).ShouldNot(Succeed())

						By("setting the status condition 'Ready' to 'true' ", func() {
							EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Conditions
							}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
								"Reason": Equal("SuccessfulCreateOrUpdate"),
								"Status": Equal(corev1.ConditionTrue),
							})))
						})

						By("recreating the queue", func() {
							EventuallyWithOffset(1, func() bool {
								err := k8sClient.Get(
									ctx,
									types.NamespacedName{Name: expectedQueueName, Namespace: superStreamNamespace},
									&queue,
								)
								if err != nil {
									return false
								}
								return queue.GetUID() != initialUID
							}, 10*time.Second, 1*time.Second).Should(BeTrue())
						})
					})
				})

				When("the exchange is deleted", func() {
					BeforeEach(func() {
						superStreamName = "delete-exchange"
					})
					It("recreates the missing object", func() {
						var exchange topologyv1beta1.Exchange
						Expect(k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName + "-exchange", Namespace: superStreamNamespace},
							&exchange,
						)).To(Succeed())
						initialUID := exchange.GetUID()
						Expect(k8sClient.Delete(ctx, &exchange, runtimeClient.GracePeriodSeconds(0))).To(Succeed())
						Eventually(komega.Get(&exchange)).Within(time.Second * 10).WithPolling(time.Second).ShouldNot(Succeed())

						By("setting the status condition 'Ready' to 'true' ", func() {
							EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Conditions
							}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
								"Reason": Equal("SuccessfulCreateOrUpdate"),
								"Status": Equal(corev1.ConditionTrue),
							})))
						})

						By("recreating the exchange", func() {
							EventuallyWithOffset(1, func() bool {
								err := k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName + "-exchange", Namespace: superStreamNamespace},
									&exchange,
								)
								if err != nil {
									return false
								}
								return exchange.GetUID() != initialUID
							}, 10*time.Second, 1*time.Second).Should(BeTrue(), "exchange should have been recreated")
						})
					})
				})

				When("the super stream is scaled down", func() {
					var originalPartitionCount int
					BeforeEach(func() {
						superStreamName = "scale-down-super-stream"
					})
					It("refuses scaling down the partitions with a helpful warning", func() {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
							&superStream,
						)
						originalPartitionCount = len(superStream.Status.Partitions)
						superStream.Spec.Partitions = 1
						Expect(k8sClient.Update(ctx, &superStream)).To(Succeed())

						By("setting the status condition 'Ready' to 'false' ", func() {
							EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Conditions
							}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
								"Reason": Equal("FailedCreateOrUpdate"),
								"Status": Equal(corev1.ConditionFalse),
							})))
						})

						By("retaining the original stream queue partitions", func() {
							var partition topologyv1beta1.Queue
							expectedQueueNames = []string{}
							for i := 0; i < originalPartitionCount; i++ {
								expectedQueueName := fmt.Sprintf("%s-partition-%d", superStreamName, i)
								Expect(k8sClient.Get(
									ctx,
									types.NamespacedName{Name: expectedQueueName, Namespace: superStreamNamespace},
									&partition,
								)).To(Succeed())
								expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

								Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
									"Name":    Equal(fmt.Sprint(managedresource.RoutingKeyToPartitionName(superStreamName, strconv.Itoa(i)))),
									"Type":    Equal("stream"),
									"Durable": BeTrue(),
									"RabbitmqClusterReference": MatchAllFields(Fields{
										"Name":             Equal("example-rabbit"),
										"Namespace":        Equal(superStreamNamespace),
										"ConnectionSecret": BeNil(),
									}),
								}))
							}
						})

						By("setting the status of the super stream to list the partition queue names", func() {
							ConsistentlyWithOffset(1, func() []string {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Partitions
							}, 5*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
						})

						By("retaining the original bindings", func() {
							var binding topologyv1beta1.Binding
							for i := 0; i < originalPartitionCount; i++ {
								expectedBindingName := fmt.Sprintf("%s-binding-%d", superStreamName, i)
								EventuallyWithOffset(1, func() error {
									return k8sClient.Get(
										ctx,
										types.NamespacedName{Name: expectedBindingName, Namespace: superStreamNamespace},
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
										"Name":             Equal("example-rabbit"),
										"Namespace":        Equal(superStreamNamespace),
										"ConnectionSecret": BeNil(),
									}),
								}))
							}
						})

						By("raising a warning event", func() {
							Expect(observedEvents()).To(ContainElement("Warning FailedScaleDown SuperStreams cannot be scaled down: an attempt was made to scale from 3 partitions to 1"))
						})

					})
				})
			})

			When("the super stream is scaled", func() {
				JustBeforeEach(func() {
					Expect(k8sClient.Create(ctx, &superStream)).To(Succeed())
					EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
							&superStream,
						)

						return superStream.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
				})
				When("the super stream is scaled out", func() {
					BeforeEach(func() {
						superStreamName = "scale-out-super-stream"
					})
					It("allows the number of partitions to be increased", func() {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
							&superStream,
						)
						superStream.Spec.Partitions = 5
						Expect(k8sClient.Update(ctx, &superStream)).To(Succeed())

						By("creating n stream queue partitions", func() {
							var partition topologyv1beta1.Queue
							expectedQueueNames = []string{}
							for i := 0; i < superStream.Spec.Partitions; i++ {
								expectedQueueName := fmt.Sprintf("%s-partition-%s", superStreamName, strconv.Itoa(i))
								EventuallyWithOffset(1, func() error {
									return k8sClient.Get(
										ctx,
										types.NamespacedName{Name: expectedQueueName, Namespace: superStreamNamespace},
										&partition,
									)
								}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
								expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

								Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
									"Name":    Equal(fmt.Sprint(managedresource.RoutingKeyToPartitionName(superStreamName, strconv.Itoa(i)))),
									"Type":    Equal("stream"),
									"Durable": BeTrue(),
									"RabbitmqClusterReference": MatchAllFields(Fields{
										"Name":             Equal("example-rabbit"),
										"Namespace":        Equal(superStreamNamespace),
										"ConnectionSecret": BeNil(),
									}),
								}))
							}
						})

						By("setting the status of the super stream to list the partition queue names", func() {
							EventuallyWithOffset(1, func() []string {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Partitions
							}, 10*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
						})

						By("creating n bindings", func() {
							var binding topologyv1beta1.Binding
							for i := 0; i < superStream.Spec.Partitions; i++ {
								expectedBindingName := fmt.Sprintf("%s-binding-%s", superStreamName, strconv.Itoa(i))
								EventuallyWithOffset(1, func() error {
									return k8sClient.Get(
										ctx,
										types.NamespacedName{Name: expectedBindingName, Namespace: superStreamNamespace},
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
										"Name":             Equal("example-rabbit"),
										"Namespace":        Equal(superStreamNamespace),
										"ConnectionSecret": BeNil(),
									}),
								}))
							}
						})

						By("setting the status condition 'Ready' to 'true' ", func() {
							EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
								_ = k8sClient.Get(
									ctx,
									types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
									&superStream,
								)

								return superStream.Status.Conditions
							}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
								"Reason": Equal("SuccessfulCreateOrUpdate"),
								"Status": Equal(corev1.ConditionTrue),
							})))
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
					Expect(k8sClient.Create(ctx, &superStream)).To(Succeed())

					By("setting the status condition 'Ready' to 'true' ", func() {
						EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
							var fetchedSuperStream topologyv1alpha1.SuperStream
							_ = k8sClient.Get(
								ctx,
								types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
								&fetchedSuperStream,
							)

							return fetchedSuperStream.Status.Conditions
						}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topologyv1beta1.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					})

					By("creating an exchange", func() {
						var exchange topologyv1beta1.Exchange
						err := k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName + "-exchange", Namespace: superStreamNamespace},
							&exchange,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(exchange.Spec).To(MatchFields(IgnoreExtras, Fields{
							"Name":    Equal(superStreamName),
							"Type":    Equal("direct"),
							"Durable": BeTrue(),
							"RabbitmqClusterReference": MatchAllFields(Fields{
								"Name":             Equal("example-rabbit"),
								"Namespace":        Equal(superStreamNamespace),
								"ConnectionSecret": BeNil(),
							}),
						}))
					})

					By("creating n stream queue partitions", func() {
						var partition topologyv1beta1.Queue
						expectedQueueNames = []string{}
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedQueueName := fmt.Sprintf("%s-partition-%d", superStreamName, i)
							err := k8sClient.Get(
								ctx,
								types.NamespacedName{Name: expectedQueueName, Namespace: superStreamNamespace},
								&partition,
							)
							Expect(err).NotTo(HaveOccurred())

							expectedQueueNames = append(expectedQueueNames, partition.Spec.Name)

							Expect(partition.Spec).To(MatchFields(IgnoreExtras, Fields{
								"Name":    Equal(fmt.Sprintf("%s-%s", superStreamName, superStream.Spec.RoutingKeys[i])),
								"Type":    Equal("stream"),
								"Durable": BeTrue(),
								"RabbitmqClusterReference": MatchAllFields(Fields{
									"Name":             Equal("example-rabbit"),
									"Namespace":        Equal(superStreamNamespace),
									"ConnectionSecret": BeNil(),
								}),
							}))
						}
					})

					By("setting the status of the super stream to list the partition queue names", func() {
						EventuallyWithOffset(1, func() []string {
							_ = k8sClient.Get(
								ctx,
								types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
								&superStream,
							)

							return superStream.Status.Partitions
						}, 10*time.Second, 1*time.Second).Should(ConsistOf(expectedQueueNames))
					})

					By("creating n bindings", func() {
						var binding topologyv1beta1.Binding
						for i := 0; i < superStream.Spec.Partitions; i++ {
							expectedBindingName := fmt.Sprintf("%s-binding-%d", superStreamName, i)
							err := k8sClient.Get(
								ctx,
								types.NamespacedName{Name: expectedBindingName, Namespace: superStreamNamespace},
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
									"Name":             Equal("example-rabbit"),
									"Namespace":        Equal(superStreamNamespace),
									"ConnectionSecret": BeNil(),
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
					Expect(k8sClient.Create(ctx, &superStream)).To(Succeed())
					EventuallyWithOffset(1, func() []topologyv1beta1.Condition {
						var fetchedSuperStream topologyv1alpha1.SuperStream
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: superStreamName, Namespace: superStreamNamespace},
							&fetchedSuperStream,
						)

						return fetchedSuperStream.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topologyv1beta1.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Message": Equal("SuperStream mismatch failed to reconcile"),
						"Status":  Equal(corev1.ConditionFalse),
					})))
				})
			})
		})
	})
})
