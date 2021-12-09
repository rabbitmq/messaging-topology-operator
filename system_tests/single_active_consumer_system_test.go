package system_tests

import (
	"context"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("SuperStream", func() {
	var (
		namespace               = MustHaveEnv("NAMESPACE")
		ctx                     = context.Background()
		vhost                   *topology.Vhost
		vhostName               string
		superStream             *topology.SuperStream
		superStreamName         string
		superStreamConsumer     *topology.SuperStreamConsumer
		superStreamConsumerName string
	)

	JustBeforeEach(func() {
		vhost = &topology.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vhostName,
				Namespace: namespace,
			},
			Spec: topology.VhostSpec{
				Name: vhostName,
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
		superStream = &topology.SuperStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      superStreamName,
				Namespace: namespace,
			},
			Spec: topology.SuperStreamSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Name:       superStreamName,
				Vhost:      vhostName,
				Partitions: 4,
				RoutingKeys: []string{
					"eu-west-1",
					"eu-west-2",
					"eu-west-3",
					"eu-west-4",
				},
			},
		}
		superStreamConsumer = &topology.SuperStreamConsumer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      superStreamConsumerName,
				Namespace: namespace,
			},
			Spec: topology.SuperStreamConsumerSpec{
				SuperStreamReference: topology.SuperStreamReference{
					Name:      superStreamName,
					Namespace: namespace,
				},
				ConsumerPodSpec: topology.SuperStreamConsumerPodSpec{
					Default: &corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    "super-stream-app",
								Image:   "pivotalrabbitmq/super-stream-app@sha256:5ae5d72e748dd5f742d30a111b279a79b8781430647668160531edcdfe85067b",
								Command: []string{"bash", "-c"},
								Args: []string{`STREAM_URI="rabbitmq-stream://$(cat /etc/rabbitmq-creds/username):$(cat /etc/rabbitmq-creds/password)@$(cat /etc/rabbitmq-creds/host):5552/%2f";
ACTIVE_PARTITION="$(cat /etc/podinfo/active_partition_consumer)";
java -Dio.netty.processId=1 -jar super-stream-app.jar consumer --stream "${ACTIVE_PARTITION}" --stream-uri "${STREAM_URI}" ;`},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "podinfo",
										MountPath: "/etc/podinfo",
										ReadOnly:  true,
									},
									{
										Name:      "rabbitmq-creds",
										MountPath: "/etc/rabbitmq-creds",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "podinfo",
								VolumeSource: corev1.VolumeSource{
									DownwardAPI: &corev1.DownwardAPIVolumeSource{
										Items: []corev1.DownwardAPIVolumeFile{
											{
												Path: "active_partition_consumer",
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.labels['rabbitmq.com/super-stream-partition']",
												},
											},
										},
									},
								},
							},
							{
								Name: "rabbitmq-creds",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "system-test-default-user",
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, vhost)).To(Succeed())
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, superStream)
		_ = k8sClient.Delete(ctx, superStreamConsumer)
		_, _ = kubectl(
			"-n",
			namespace,
			"delete",
			"-f",
			"../system_tests/fixtures/container-kill.yaml",
			"--ignore-not-found",
		)
		_ = k8sClient.Delete(ctx, vhost)
	})

	When("just creating a superstream", func() {
		BeforeEach(func() {
			superStreamName = "super-stream-test"
			vhostName = "super-vhost-1"
		})
		It("creates and deletes a superStream successfully", func() {
			By("creating an exchange")
			Expect(k8sClient.Create(ctx, superStream, &client.CreateOptions{})).To(Succeed())
			var exchangeInfo *rabbithole.DetailedExchangeInfo
			Eventually(func() error {
				var err error
				exchangeInfo, err = rabbitClient.GetExchange(vhostName, superStreamName)
				return err
			}, 30, 2).Should(BeNil())

			Expect(*exchangeInfo).To(MatchFields(IgnoreExtras, Fields{
				"Name":       Equal("super-stream-test"),
				"Vhost":      Equal(vhostName),
				"Type":       Equal("direct"),
				"AutoDelete": BeFalse(),
				"Durable":    BeTrue(),
			}))

			By("creating n queues")
			for _, routingKey := range superStream.Spec.RoutingKeys {
				var qInfo *rabbithole.DetailedQueueInfo
				Eventually(func() error {
					var err error
					qInfo, err = rabbitClient.GetQueue(vhostName, fmt.Sprintf("super-stream-test-%s", routingKey))
					return err
				}, 10, 2).Should(BeNil())

				Expect(*qInfo).To(MatchFields(IgnoreExtras, Fields{
					"Name":       Equal(fmt.Sprintf("super-stream-test-%s", routingKey)),
					"Vhost":      Equal(vhostName),
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
					bindings, err := rabbitClient.ListBindingsIn(vhostName)
					Expect(err).NotTo(HaveOccurred())
					for _, b := range bindings {
						if b.Source == "super-stream-test" && b.Destination == fmt.Sprintf("super-stream-test-%s", routingKey) {
							fetchedBinding = b
							return true
						}
					}
					return false
				}, 10, 2).Should(BeTrue(), "cannot find created binding")
				Expect(fetchedBinding).To(MatchFields(IgnoreExtras, Fields{
					"Vhost":           Equal(vhostName),
					"Source":          Equal("super-stream-test"),
					"Destination":     Equal(fmt.Sprintf("super-stream-test-%s", routingKey)),
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
				_, err = rabbitClient.GetExchange(vhostName, "super-stream-test")
				return err
			}, 10).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Object Not Found"))

			By("deleting underlying resources")
			Eventually(func() error {
				_, err = rabbitClient.GetExchange(vhostName, "super-stream-test")
				return err
			}, 10, 2).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Object Not Found"))

			for _, routingKey := range superStream.Spec.RoutingKeys {
				Eventually(func() error {
					_, err = rabbitClient.GetQueue(vhostName, fmt.Sprintf("super-stream-test-%s", routingKey))
					return err
				}, 10, 2).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Object Not Found"))

				Eventually(func() bool {
					bindings, err := rabbitClient.ListBindingsIn(vhostName)
					Expect(err).NotTo(HaveOccurred())
					for _, b := range bindings {
						if b.Source == "super-stream-test" && b.Destination == fmt.Sprintf("super-stream-test-%s", routingKey) {
							return true
						}
					}
					return false
				}, 10, 2).Should(BeFalse(), "found the binding where we expected it to have been deleted")
			}
		})
	})

	When("creating a super stream and a consumer for the superstream", func() {
		JustBeforeEach(func() {
			Expect(k8sClient.Create(ctx, superStream, &client.CreateOptions{})).To(Succeed())
			Eventually(func() topology.Condition {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStream.Name, Namespace: superStream.Namespace}, superStream)).To(Succeed())
				if len(superStream.Status.Conditions) != 1 {
					return topology.Condition{}
				}
				return superStream.Status.Conditions[0]
			}, 60*time.Second, 1*time.Second).Should(MatchFields(IgnoreExtras, Fields{
				"Type":               Equal(topology.ConditionType("Ready")),
				"Status":             Equal(corev1.ConditionTrue),
				"Reason":             Equal("SuccessfulCreateOrUpdate"),
				"LastTransitionTime": Not(Equal(metav1.Time{})),
			}))
			Expect(k8sClient.Create(ctx, superStreamConsumer, &client.CreateOptions{})).To(Succeed())
			Eventually(func() topology.Condition {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStreamConsumer.Name, Namespace: superStreamConsumer.Namespace}, superStreamConsumer)).To(Succeed())
				if len(superStreamConsumer.Status.Conditions) != 1 {
					return topology.Condition{}
				}
				return superStreamConsumer.Status.Conditions[0]
			}, 60*time.Second, 1*time.Second).Should(MatchFields(IgnoreExtras, Fields{
				"Type":               Equal(topology.ConditionType("Ready")),
				"Status":             Equal(corev1.ConditionTrue),
				"Reason":             Equal("SuccessfulCreateOrUpdate"),
				"LastTransitionTime": Not(Equal(metav1.Time{})),
			}))
		})
		When("creating both objects", func() {
			BeforeEach(func() {
				superStreamName = "topology-test"
				superStreamConsumerName = "topology-consumer"
				vhostName = "super-vhost-2"
			})
			It("creates a consumer pod for each partition in the super stream", func() {
				containerName := func(element interface{}) string {
					return element.(corev1.Container).Name
				}
				volumeName := func(element interface{}) string {
					return element.(corev1.Volume).Name
				}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStream.Name, Namespace: superStream.Namespace}, superStream)).To(Succeed())
				for _, partition := range superStream.Status.Partitions {
					consumerPod := getSingleActiveConsumerPod(ctx, superStream, partition)
					Expect(consumerPod.Spec.Containers).To(MatchAllElements(containerName, Elements{
						"super-stream-app": Not(BeZero()),
					}))
					Expect(consumerPod.Spec.Volumes).To(MatchElements(volumeName, IgnoreExtras, Elements{
						"podinfo":        Not(BeZero()),
						"rabbitmq-creds": Not(BeZero()),
					}))
				}
			})
		})
		When("a consumer pod is deleted", func() {
			var targetPartition string
			var deletedPod corev1.Pod
			BeforeEach(func() {
				superStreamName = "deletion-test"
				superStreamConsumerName = "deletion-consumer"
				vhostName = "super-vhost-3"
			})
			JustBeforeEach(func() {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStream.Name, Namespace: superStream.Namespace}, superStream)).To(Succeed())
				targetPartition = superStream.Status.Partitions[0]
				deletedPod = getSingleActiveConsumerPod(ctx, superStream, targetPartition)
				Expect(k8sClient.Delete(ctx, &deletedPod)).To(Succeed())
			})
			It("recreates the pod with a regenerated pod name", func() {
				Eventually(func() corev1.Pod {
					var consumerPods corev1.PodList
					Expect(k8sClient.List(ctx, &consumerPods, client.InNamespace(superStream.Namespace), client.MatchingLabels(map[string]string{
						managedresource.AnnotationSuperStream:          superStream.Name,
						managedresource.AnnotationSuperStreamPartition: targetPartition,
					}))).To(Succeed())
					if len(consumerPods.Items) != 1 {
						return corev1.Pod{}
					}
					return consumerPods.Items[0]
				}, 30*time.Second, 1*time.Second).Should(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Name":         Not(Equal(deletedPod.Name)),
						"GenerateName": Equal(deletedPod.GenerateName),
						"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Equal(deletedPod.Labels[managedresource.AnnotationConsumerPodSpecHash])),
					}),
				}))
			})
		})
		When("a consumer pod container hits an error", func() {
			BeforeEach(func() {
				superStreamName = "error-test"
				superStreamConsumerName = "error-consumer"
				vhostName = "super-vhost-4"
			})
			It("recreates the container in the same Pod", func() {
				if !environmentHasChaosMeshInstalled {
					Skip("chaos mesh is not installed on this test cluster so cannot simulate container failure")
				}
				containerStatusName := func(element interface{}) string {
					return element.(corev1.ContainerStatus).Name
				}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStream.Name, Namespace: superStream.Namespace}, superStream)).To(Succeed())
				targetPartition := managedresource.RoutingKeyToPartitionName(superStream.Name, "eu-west-2")
				targetPod := getSingleActiveConsumerPod(ctx, superStream, targetPartition)

				output, err := kubectl(
					"-n",
					namespace,
					"create",
					"-f",
					"../system_tests/fixtures/container-kill.yaml",
				)
				Expect(err).NotTo(HaveOccurred(), string(output))

				Eventually(func() corev1.Pod {
					var foundPod corev1.Pod
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: targetPod.Name, Namespace: targetPod.Namespace}, &foundPod)).To(Succeed())
					return foundPod
				}, 30*time.Second, 1*time.Second).Should(MatchFields(IgnoreExtras, Fields{
					"Status": MatchFields(IgnoreExtras, Fields{
						"ContainerStatuses": MatchAllElements(containerStatusName, Elements{
							"super-stream-app": MatchFields(IgnoreExtras, Fields{
								"RestartCount": Equal(int32(1)),
							}),
						}),
					}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Name":         Equal(targetPod.Name),
						"GenerateName": Equal(targetPod.GenerateName),
						"Labels":       HaveKeyWithValue(managedresource.AnnotationConsumerPodSpecHash, Equal(targetPod.Labels[managedresource.AnnotationConsumerPodSpecHash])),
					}),
				}))
			})
		})
		When("the super stream scales up its number of partitions", func() {
			var existingPods corev1.PodList
			BeforeEach(func() {
				superStreamName = "scaling-test"
				superStreamConsumerName = "scaling-consumer"
				vhostName = "super-vhost-5"
			})
			JustBeforeEach(func() {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStream.Name, Namespace: superStream.Namespace}, superStream)).To(Succeed())
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: superStreamConsumer.Name, Namespace: superStreamConsumer.Namespace}, superStreamConsumer)).To(Succeed())
				Expect(k8sClient.List(ctx, &existingPods, client.InNamespace(superStream.Namespace), client.MatchingLabels(map[string]string{
					managedresource.AnnotationSuperStream: superStream.Name,
				}))).To(Succeed())
				Expect(len(existingPods.Items)).To(Equal(4))
				superStream.Spec.Partitions = 5
				superStream.Spec.RoutingKeys = append(superStream.Spec.RoutingKeys, "eu-west-5")
				Expect(k8sClient.Update(ctx, superStream)).To(Succeed())
			})
			It("scales the topology", func() {
				By("creating an extra binding and partition queue")
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "scaling-test-partition-4", Namespace: superStream.Namespace}, &topology.Queue{})
				}, 30*time.Second, 1*time.Second).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "scaling-test-binding-4", Namespace: superStream.Namespace}, &topology.Binding{})
				}, 30*time.Second, 1*time.Second).Should(Succeed())

				By("creating an extra consumer pod")
				Eventually(func() error {
					var consumerPods corev1.PodList
					targetPartition := managedresource.RoutingKeyToPartitionName(superStream.Name, "eu-west-5")
					Expect(k8sClient.List(ctx, &consumerPods, client.InNamespace(superStream.Namespace), client.MatchingLabels(map[string]string{
						managedresource.AnnotationSuperStream:          superStream.Name,
						managedresource.AnnotationSuperStreamPartition: targetPartition,
					}))).To(Succeed())
					if len(consumerPods.Items) != 1 {
						return fmt.Errorf("Cannot find pod for routing key eu-west-5")
					}
					return nil
				}, 30*time.Second, 1*time.Second).Should(Succeed())

				By("not touching the existing consumer pods")
				for _, existingPod := range existingPods.Items {
					pod := getSingleActiveConsumerPod(ctx, superStream, existingPod.Labels[managedresource.AnnotationSuperStreamPartition])
					Expect(pod.Name).To(Equal(existingPod.Name))
					Expect(pod.Status.ContainerStatuses[0].RestartCount).To(BeZero())
				}

			})
		})
	})
})

func getSingleActiveConsumerPod(ctx context.Context, superStream *topology.SuperStream, partitionName string) corev1.Pod {
	var consumerPods corev1.PodList
	Expect(k8sClient.List(ctx, &consumerPods, client.InNamespace(superStream.Namespace), client.MatchingLabels(map[string]string{
		managedresource.AnnotationSuperStream:          superStream.Name,
		managedresource.AnnotationSuperStreamPartition: partitionName,
	}))).To(Succeed())
	Expect(len(consumerPods.Items)).To(Equal(1), fmt.Sprintf("Expected for exactly one Pod to have been created for partition %s, but saw %d", superStream.Status.Partitions[0], len(consumerPods.Items)))
	return consumerPods.Items[0]
}
