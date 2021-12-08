package controllers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"time"
)

func byNamePrefixPodSorter(pods []corev1.Pod) ([]corev1.Pod, func(i, j int) bool) {
	return pods, func(i, j int) bool { return pods[i].GenerateName < pods[j].GenerateName }
}

var _ = Describe("super-stream-consumer-controller", func() {

	var superStream topology.SuperStream
	var superStreamName string
	var superStreamConsumer topology.SuperStreamConsumer
	var superStreamConsumerName string
	var partitions int
	var routingKeys []string
	var consumerPodSpec topology.SuperStreamConsumerPodSpec

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
			superStreamConsumer = topology.SuperStreamConsumer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      superStreamConsumerName,
					Namespace: "default",
				},
				Spec: topology.SuperStreamConsumerSpec{
					SuperStreamReference: topology.SuperStreamReference{
						Name: superStreamName,
					},
					ConsumerPodSpec: consumerPodSpec,
				},
			}
			superStream = topology.SuperStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      superStreamName,
					Namespace: "default",
				},
				Spec: topology.SuperStreamSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
					Partitions:  partitions,
					RoutingKeys: routingKeys,
				},
			}
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

		Context("creation", func() {
			When("success", func() {
				BeforeEach(func() {
					superStreamName = "example-stream"
					superStreamConsumerName = "basic-consumer"
					partitions = 2
					routingKeys = nil
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						Default: &corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "my-container",
									Image: "my-image",
								},
							},
						},
					}
				})

				It("creates the SuperStreamConsumer and any underlying resources", func() {
					Expect(client.Create(ctx, &superStreamConsumer)).To(Succeed())

					By("setting the status condition 'Ready' to 'true' ", func() {
						EventuallyWithOffset(1, func() []topology.Condition {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: superStreamConsumerName, Namespace: "default"},
								&superStreamConsumer,
							)

							return superStreamConsumer.Status.Conditions
						}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					})
					By("creating a Pod for each partition in the SuperStream, multiplied by the number of replicas", func() {
						for _, partition := range superStream.Status.Partitions {
							var pod corev1.Pod
							var err error
							EventuallyWithOffset(1, func() error {
								pod, err = getActiveConsumerPod(ctx, superStream, partition)
								return err
							}, 10*time.Second, 1*time.Second).Should(Succeed())

							Expect(pod.Spec.Containers[0].Name).To(Equal(superStreamConsumer.Spec.ConsumerPodSpec.Default.Containers[0].Name))
							Expect(pod.Spec.Containers[0].Image).To(Equal(superStreamConsumer.Spec.ConsumerPodSpec.Default.Containers[0].Image))
							Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStream, superStream.Name))
							Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStreamPartition, partition))
						}
					})
				})
			})
		})

		Context("pod spec modification", func() {
			var podList corev1.PodList
			JustBeforeEach(func() {
				Expect(client.Create(ctx, &superStreamConsumer)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: superStreamConsumerName, Namespace: "default"},
						&superStreamConsumer,
					)

					return superStreamConsumer.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))

				Expect(client.List(ctx, &podList, runtimeClient.InNamespace(superStreamConsumer.Namespace), runtimeClient.MatchingLabels(map[string]string{
					managedresource.AnnotationSuperStream: superStreamName,
				}))).To(Succeed())
				sort.Slice(byNamePrefixPodSorter(podList.Items))
			})

			When("the default podSpec changes", func() {
				BeforeEach(func() {
					superStreamName = "modify"
					superStreamConsumerName = "default-change"
					routingKeys = nil
					partitions = 3
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						Default: &corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "my-container",
									Image: "my-image",
								},
							},
						},
					}
				})

				It("creates new pods with the updated podSpec and deletes the old ones", func() {
					superStreamConsumer.Spec.ConsumerPodSpec.Default.Containers[0].Image = "my-updated-image"
					Expect(client.Update(ctx, &superStreamConsumer)).To(Succeed())
					var updatedPodList corev1.PodList
					EventuallyWithOffset(1, func() string {
						err := client.List(ctx, &updatedPodList, runtimeClient.InNamespace(superStreamConsumer.Namespace), runtimeClient.MatchingLabels(map[string]string{
							managedresource.AnnotationSuperStream: superStreamName,
						}))
						Expect(err).NotTo(HaveOccurred())
						sort.Slice(byNamePrefixPodSorter(updatedPodList.Items))
						return updatedPodList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]
					}, 10*time.Second, 1*time.Second).ShouldNot(Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]))

					Expect(len(podList.Items)).To(Equal(len(updatedPodList.Items)))
					for i := 0; i < len(updatedPodList.Items); i++ {
						Expect(updatedPodList.Items[i]).To(MatchFields(IgnoreExtras, Fields{
							"ObjectMeta": MatchFields(IgnoreExtras, Fields{
								"Name":         Not(Equal(podList.Items[i].Name)),
								"GenerateName": Equal(podList.Items[i].GenerateName),
								"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Not(Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]))),
							}),
						}))
						Expect(updatedPodList.Items[i].Spec.Containers[0].Image).To(Equal("my-updated-image"))
					}
				})
			})

			When("a routing-key-specific podSpec is removed", func() {
				BeforeEach(func() {
					superStreamName = "removal"
					superStreamConsumerName = "routing-key-removal"
					partitions = 3
					routingKeys = []string{"foo", "bar", "baz"}
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						Default: &corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "default-container",
									Image: "default-image",
								},
							},
						},
						PerRoutingKey: map[string]*corev1.PodSpec{
							"foo": {
								Containers: []corev1.Container{
									{
										Name:  "specific-container",
										Image: "specific-image",
									},
								},
							},
							"bar": {
								Containers: []corev1.Container{
									{
										Name:  "specific-container",
										Image: "specific-image",
									},
								},
							},
							"baz": {
								Containers: []corev1.Container{
									{
										Name:  "specific-container",
										Image: "specific-image",
									},
								},
							},
						},
					}
				})

				It("reverts the affected pod to the default podSpec", func() {
					superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey = map[string]*corev1.PodSpec{
						"foo": {
							Containers: []corev1.Container{
								{
									Name:  "specific-container",
									Image: "specific-image",
								},
							},
						},
						"bar": {
							Containers: []corev1.Container{
								{
									Name:  "specific-container",
									Image: "specific-image",
								},
							},
						},
					}
					Expect(client.Update(ctx, &superStreamConsumer)).To(Succeed())

					var updatedPodList corev1.PodList
					EventuallyWithOffset(1, func() string {
						err := client.List(ctx, &updatedPodList, runtimeClient.InNamespace(superStreamConsumer.Namespace), runtimeClient.MatchingLabels(map[string]string{
							managedresource.AnnotationSuperStream: superStreamName,
						}))
						Expect(err).NotTo(HaveOccurred())
						sort.Slice(byNamePrefixPodSorter(updatedPodList.Items))
						return updatedPodList.Items[1].Labels[managedresource.AnnotationConsumerPodSpecHash]
					}, 10*time.Second, 1*time.Second).ShouldNot(Equal(podList.Items[1].Labels[managedresource.AnnotationConsumerPodSpecHash]))

					Expect(len(podList.Items)).To(Equal(len(updatedPodList.Items)))
					for i := 0; i < len(updatedPodList.Items); i++ {
						if updatedPodList.Items[i].Labels[managedresource.AnnotationSuperStreamPartition] == "removal-baz" {
							Expect(updatedPodList.Items[i]).To(MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": MatchFields(IgnoreExtras, Fields{
									"Name":         Not(Equal(podList.Items[i].Name)),
									"GenerateName": Equal(podList.Items[i].GenerateName),
									"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Not(Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]))),
								}),
							}))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Name).To(Equal("default-container"))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Image).To(Equal("default-image"))
						} else {
							Expect(updatedPodList.Items[i]).To(MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": MatchFields(IgnoreExtras, Fields{
									"Name":         Equal(podList.Items[i].Name),
									"GenerateName": Equal(podList.Items[i].GenerateName),
									"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash])),
								}),
							}))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Name).To(Equal("specific-container"))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Image).To(Equal("specific-image"))
						}
					}
				})
			})
			When("a routing-key-specific podSpec is removed and no default is provided", func() {
				BeforeEach(func() {
					superStreamName = "no-default"
					superStreamConsumerName = "no-default"
					partitions = 3
					routingKeys = []string{"foo", "bar", "baz"}
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						PerRoutingKey: map[string]*corev1.PodSpec{
							"foo": {
								Containers: []corev1.Container{
									{
										Name:  "specific-container",
										Image: "specific-image",
									},
								},
							},
							"bar": {
								Containers: []corev1.Container{
									{
										Name:  "specific-container",
										Image: "specific-image",
									},
								},
							},
							"baz": {
								Containers: []corev1.Container{
									{
										Name:  "specific-container",
										Image: "specific-image",
									},
								},
							},
						},
					}
				})

				It("deletes the affected pod and leaves the rest alone", func() {
					superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey = map[string]*corev1.PodSpec{
						"foo": {
							Containers: []corev1.Container{
								{
									Name:  "specific-container",
									Image: "specific-image",
								},
							},
						},
						"bar": {
							Containers: []corev1.Container{
								{
									Name:  "specific-container",
									Image: "specific-image",
								},
							},
						},
					}
					Expect(client.Update(ctx, &superStreamConsumer)).To(Succeed())

					EventuallyWithOffset(1, func() int {
						var updatedPodList corev1.PodList
						err := client.List(ctx, &updatedPodList, runtimeClient.InNamespace(superStreamConsumer.Namespace), runtimeClient.MatchingLabels(map[string]string{
							managedresource.AnnotationSuperStream: superStreamName,
						}))
						Expect(err).NotTo(HaveOccurred())
						sort.Slice(byNamePrefixPodSorter(updatedPodList.Items))
						return len(updatedPodList.Items)
					}, 10*time.Second, 1*time.Second).Should(Equal(len(podList.Items)-1))

				})
			})
			When("a routing-key-specific podSpec is provided", func() {
				BeforeEach(func() {
					superStreamName = "specific"
					superStreamConsumerName = "routing-key-upgrade"
					routingKeys = []string{"foo", "bar", "baz"}
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						Default: &corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "my-container",
									Image: "my-image",
								},
							},
						},
					}
				})

				It("upgrades the single matching partition pod to the different podSpec", func() {
					superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey = make(map[string]*corev1.PodSpec)
					superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey["bar"] = &corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "my-special-container",
								Image: "my-special-image",
							},
						},
					}
					Expect(client.Update(ctx, &superStreamConsumer)).To(Succeed())

					var updatedPodList corev1.PodList
					EventuallyWithOffset(1, func() string {
						err := client.List(ctx, &updatedPodList, runtimeClient.InNamespace(superStreamConsumer.Namespace), runtimeClient.MatchingLabels(map[string]string{
							managedresource.AnnotationSuperStream: superStreamName,
						}))
						Expect(err).NotTo(HaveOccurred())
						sort.Slice(byNamePrefixPodSorter(updatedPodList.Items))
						return updatedPodList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]
					}, 10*time.Second, 1*time.Second).ShouldNot(Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]))

					Expect(len(podList.Items)).To(Equal(len(updatedPodList.Items)))
					for i := 0; i < len(updatedPodList.Items); i++ {
						if updatedPodList.Items[i].Labels[managedresource.AnnotationSuperStreamPartition] == "specific-bar" {
							Expect(updatedPodList.Items[i]).To(MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": MatchFields(IgnoreExtras, Fields{
									"Name":         Not(Equal(podList.Items[i].Name)),
									"GenerateName": Equal(podList.Items[i].GenerateName),
									"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Not(Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash]))),
								}),
							}))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Name).To(Equal("my-special-container"))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Image).To(Equal("my-special-image"))
						} else {
							Expect(updatedPodList.Items[i]).To(MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": MatchFields(IgnoreExtras, Fields{
									"Name":         Equal(podList.Items[i].Name),
									"GenerateName": Equal(podList.Items[i].GenerateName),
									"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Equal(podList.Items[0].Labels[managedresource.AnnotationConsumerPodSpecHash])),
								}),
							}))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Name).To(Equal("my-container"))
							Expect(updatedPodList.Items[i].Spec.Containers[0].Image).To(Equal("my-image"))
						}
					}
				})
			})
		})

		Context("pod deletion", func() {
			When("a active consumer pod is deleted", func() {
				var deletedPodPartition string
				BeforeEach(func() {
					superStreamName = "delete"
					superStreamConsumerName = "active-consumer"
					partitions = 2
					routingKeys = nil
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						Default: &corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "my-container",
									Image: "my-image",
								},
							},
						},
					}
				})
				JustBeforeEach(func() {
					Expect(client.Create(ctx, &superStreamConsumer)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: superStreamConsumerName, Namespace: "default"},
							&superStreamConsumer,
						)

						return superStreamConsumer.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))

					var podList corev1.PodList
					err := client.List(ctx, &podList, runtimeClient.InNamespace(superStreamConsumer.Namespace), runtimeClient.MatchingLabels(map[string]string{
						managedresource.AnnotationSuperStream: superStreamName,
					}))
					Expect(err).NotTo(HaveOccurred())
					Expect(podList.Items).To(HaveLen(2))
					deletedPodPartition = podList.Items[0].Labels[managedresource.AnnotationSuperStreamPartition]
					Expect(client.Delete(ctx, &podList.Items[0])).To(Succeed())
				})

				It("ensures a consumer is recreated", func() {
					By("recreating the deleted Pod", func() {
						EventuallyWithOffset(1, func() error {
							_, err := getActiveConsumerPod(ctx, superStream, deletedPodPartition)
							return err
						}, 10*time.Second, 1*time.Second).Should(Succeed())
					})
					By("setting the reconcile status to successful", func() {
						EventuallyWithOffset(1, func() []topology.Condition {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: superStreamConsumerName, Namespace: "default"},
								&superStreamConsumer,
							)

							return superStreamConsumer.Status.Conditions
						}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					})
				})
			})
		})

		Context("different routing keys", func() {
			When("a different PodSpec is specified for each routing key", func() {
				BeforeEach(func() {
					superStreamName = "orders"
					superStreamConsumerName = "different-keys"
					partitions = 3
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						PerRoutingKey: map[string]*corev1.PodSpec{
							"amer": {
								Containers: []corev1.Container{
									{
										Name:  "amer-pod",
										Image: "amer-image",
									},
								},
							},
							"apj": {
								Containers: []corev1.Container{
									{
										Name:  "apj-pod",
										Image: "apj-image",
									},
								},
							},
							"emea": {
								Containers: []corev1.Container{
									{
										Name:  "emea-pod",
										Image: "emea-image",
									},
								},
							},
						},
					}
					routingKeys = []string{"amer", "apj", "emea"}
				})

				It("creates a pod for each partition with the different pod specs", func() {
					Expect(client.Create(ctx, &superStreamConsumer)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: superStreamConsumerName, Namespace: "default"},
							&superStreamConsumer,
						)

						return superStreamConsumer.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
					for _, partition := range superStream.Status.Partitions {
						var pod corev1.Pod
						var err error
						EventuallyWithOffset(1, func() error {
							pod, err = getActiveConsumerPod(ctx, superStream, partition)
							return err
						}, 10*time.Second, 1*time.Second).Should(Succeed())

						routingKey := managedresource.PartitionNameToRoutingKey(superStream.Name, partition)
						Expect(pod.Spec.Containers[0].Name).To(Equal(superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey[routingKey].Containers[0].Name))
						Expect(pod.Spec.Containers[0].Image).To(Equal(superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey[routingKey].Containers[0].Image))
						Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStream, superStream.Name))
						Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStreamPartition, partition))
					}
				})
			})
			When("a PodSpec is missing for a given routing key", func() {
				BeforeEach(func() {
					superStreamName = "missing"
					superStreamConsumerName = "several-keys"
					partitions = 3
					consumerPodSpec = topology.SuperStreamConsumerPodSpec{
						PerRoutingKey: map[string]*corev1.PodSpec{
							"amer": {
								Containers: []corev1.Container{
									{
										Name:  "amer-pod",
										Image: "amer-image",
									},
								},
							},
							"emea": {
								Containers: []corev1.Container{
									{
										Name:  "emea-pod",
										Image: "emea-image",
									},
								},
							},
						},
					}
					routingKeys = []string{"amer", "apj", "emea"}
				})

				It("creates a pod for each partition that has a podSpec", func() {
					Expect(client.Create(ctx, &superStreamConsumer)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: superStreamConsumerName, Namespace: "default"},
							&superStreamConsumer,
						)

						return superStreamConsumer.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
					for _, partition := range superStream.Status.Partitions {
						var pod corev1.Pod
						var err error
						if partition == "missing-apj" {
							ConsistentlyWithOffset(1, func() error {
								pod, err = getActiveConsumerPod(ctx, superStream, partition)
								return err
							}, 10*time.Second, 1*time.Second).ShouldNot(Succeed())
						} else {
							EventuallyWithOffset(1, func() error {
								pod, err = getActiveConsumerPod(ctx, superStream, partition)
								return err
							}, 10*time.Second, 1*time.Second).Should(Succeed())

							routingKey := managedresource.PartitionNameToRoutingKey(superStream.Name, partition)
							Expect(pod.Spec.Containers[0].Name).To(Equal(superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey[routingKey].Containers[0].Name))
							Expect(pod.Spec.Containers[0].Image).To(Equal(superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey[routingKey].Containers[0].Image))
							Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStream, superStream.Name))
							Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStreamPartition, partition))
						}
					}
				})
			})
		})
	})
})
