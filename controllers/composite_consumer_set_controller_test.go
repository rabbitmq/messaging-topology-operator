package controllers_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/leaderelection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"strconv"
	"time"
)

var _ = Describe("composite-consumer-set-controller", func() {

	var superStream topology.SuperStream
	var superStreamName = "example-super-stream"
	var compositeConsumerSet topology.CompositeConsumerSet
	var compositeConsumerSetName string
	var observedPods []corev1.Pod

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
			compositeConsumerSet = topology.CompositeConsumerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      compositeConsumerSetName,
					Namespace: "default",
				},
				Spec: topology.CompositeConsumerSetSpec{
					SuperStreamReference: topology.SuperStreamReference{
						Name: superStreamName,
					},
					Replicas: 3,
					ConsumerPodSpec: topology.CompositeConsumerPodSpec{
						Default: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "my-container",
									Image: "my-image",
								},
							},
						},
					},
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
					Partitions: 2,
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
					compositeConsumerSetName = "basic-consumer-set"
				})

				It("creates the CompositeConsumerSet and any underlying resources", func() {
					Expect(client.Create(ctx, &compositeConsumerSet)).To(Succeed())

					By("setting the status condition 'Ready' to 'true' ", func() {
						EventuallyWithOffset(1, func() []topology.Condition {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: compositeConsumerSetName, Namespace: "default"},
								&compositeConsumerSet,
							)

							return compositeConsumerSet.Status.Conditions
						}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					})
					By("creating a Pod for each partition in the SuperStream, multiplied by the number of replicas", func() {
						var pod corev1.Pod
						for i := 0; i < compositeConsumerSet.Spec.Replicas; i++ {
							for j := 0; j < superStream.Spec.Partitions; j++ {
								expectedPodName := fmt.Sprintf("%s-%s-%d", compositeConsumerSetName, superStream.Status.Partitions[j], i)
								err := client.Get(
									ctx,
									types.NamespacedName{Name: expectedPodName, Namespace: "default"},
									&pod,
								)
								Expect(err).NotTo(HaveOccurred())

								observedPods = append(observedPods, pod)

								Expect(pod.Spec.Containers[0].Name).To(Equal(compositeConsumerSet.Spec.ConsumerPodSpec.Default.Containers[0].Name))
								Expect(pod.Spec.Containers[0].Image).To(Equal(compositeConsumerSet.Spec.ConsumerPodSpec.Default.Containers[0].Image))
								Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream-partition", superStream.Status.Partitions[j]))
								Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/composite-consumer-replica", strconv.Itoa(i)))
								Expect(pod.ObjectMeta.Labels).To(HaveKey("rabbitmq.com/active-partition-consumer"))
							}
						}
					})
					By("performing a leader election such that only one consumer is active per partition", func() {
						var observedActiveConsumers int
						for _, pod := range observedPods {
							if pod.ObjectMeta.Labels[leaderelection.AnnotationActiveConsumer] != "" {
								Expect(pod.ObjectMeta.Labels[leaderelection.AnnotationActiveConsumer]).To(Equal(pod.ObjectMeta.Labels[leaderelection.AnnotationPartition]))
								observedActiveConsumers += 1
							}
						}
						Expect(observedActiveConsumers).To(Equal(superStream.Spec.Partitions))
					})
				})
			})
		})
		Context("pod deletion", func() {
			var activeConsumerPod *corev1.Pod
			When("a active consumer pod is deleted", func() {
				BeforeEach(func() {
					compositeConsumerSetName = "active-consumer-delete"
				})
				JustBeforeEach(func() {
					Expect(client.Create(ctx, &compositeConsumerSet)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: compositeConsumerSetName, Namespace: "default"},
							&compositeConsumerSet,
						)

						return compositeConsumerSet.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))

					var pod corev1.Pod
					for i := 0; i < compositeConsumerSet.Spec.Replicas; i++ {
						for j := 0; j < superStream.Spec.Partitions; j++ {
							expectedPodName := fmt.Sprintf("%s-%s-%d", compositeConsumerSetName, superStream.Status.Partitions[j], i)
							err := client.Get(
								ctx,
								types.NamespacedName{Name: expectedPodName, Namespace: "default"},
								&pod,
							)
							Expect(err).NotTo(HaveOccurred())
							if pod.ObjectMeta.Labels[leaderelection.AnnotationActiveConsumer] != "" {
								activeConsumerPod = &pod
							}
						}
					}
					Expect(activeConsumerPod).NotTo(BeNil())
					Expect(client.Delete(ctx, activeConsumerPod)).To(Succeed())
				})

				FIt("elects a new leader replica consumer", func() {
					By("recreating the deleted Pod", func() {
						var deletedPod corev1.Pod
						EventuallyWithOffset(1, func() []corev1.PodCondition {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: activeConsumerPod.Name, Namespace: activeConsumerPod.Namespace},
								&deletedPod,
							)

							return deletedPod.Status.Conditions
						}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(corev1.ContainersReady),
							"Status": Equal(corev1.ConditionTrue),
						})))
						//	var observedActiveConsumers int
						//	var pod corev1.Pod
						//	for i := 0; i < compositeConsumerSet.Spec.Replicas; i++ {
						//		for j := 0; j < superStream.Spec.Partitions; j++ {
						//			expectedPodName := fmt.Sprintf("%s-%s-%d", compositeConsumerSetName, superStream.Status.Partitions[j], i)
						//			err := client.Get(
						//				ctx,
						//				types.NamespacedName{Name: expectedPodName, Namespace: "default"},
						//				&pod,
						//			)
						//			Expect(err).NotTo(HaveOccurred())
						//
						//			if pod.ObjectMeta.Labels[leaderelection.AnnotationActiveConsumer] != "" {
						//				Expect(pod.ObjectMeta.Labels[leaderelection.AnnotationActiveConsumer]).To(Equal(pod.ObjectMeta.Labels[leaderelection.AnnotationPartition]))
						//				observedActiveConsumers += 1
						//			}
						//		}
						//	}
						//	for _, pod := range observedPods {
						//	}
						//	Expect(observedActiveConsumers).To(Equal(superStream.Spec.Partitions))
						//})
						//By("performing a leader election such that only one consumer is active per partition", func() {
					})
				})
			})
		})
	})
})
