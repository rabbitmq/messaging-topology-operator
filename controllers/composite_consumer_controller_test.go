package controllers_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("composite-consumer-controller", func() {

	var superStream topology.SuperStream
	var superStreamName = "example-super-stream"
	var compositeConsumer topology.CompositeConsumer
	var compositeConsumerName string
	var observedPods []corev1.Pod

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
			compositeConsumer = topology.CompositeConsumer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      compositeConsumerName,
					Namespace: "default",
				},
				Spec: topology.CompositeConsumerSpec{
					SuperStreamReference: topology.SuperStreamReference{
						Name: superStreamName,
					},
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
					compositeConsumerName = "basic-consumer"
				})

				It("creates the CompositeConsumer and any underlying resources", func() {
					Expect(client.Create(ctx, &compositeConsumer)).To(Succeed())

					By("setting the status condition 'Ready' to 'true' ", func() {
						EventuallyWithOffset(1, func() []topology.Condition {
							_ = client.Get(
								ctx,
								types.NamespacedName{Name: compositeConsumerName, Namespace: "default"},
								&compositeConsumer,
							)

							return compositeConsumer.Status.Conditions
						}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					})
					By("creating a Pod for each partition in the SuperStream, multiplied by the number of replicas", func() {
						var pod corev1.Pod
						for j := 0; j < superStream.Spec.Partitions; j++ {
							expectedPodName := fmt.Sprintf("%s-%s", compositeConsumerName, superStream.Status.Partitions[j])
							err := client.Get(
								ctx,
								types.NamespacedName{Name: expectedPodName, Namespace: "default"},
								&pod,
							)
							Expect(err).NotTo(HaveOccurred())

							observedPods = append(observedPods, pod)

							Expect(pod.Spec.Containers[0].Name).To(Equal(compositeConsumer.Spec.ConsumerPodSpec.Default.Containers[0].Name))
							Expect(pod.Spec.Containers[0].Image).To(Equal(compositeConsumer.Spec.ConsumerPodSpec.Default.Containers[0].Image))
							Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStream, superStream.Name))
							Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(managedresource.AnnotationSuperStreamPartition, superStream.Status.Partitions[j]))
						}
					})
				})
			})
		})
		Context("pod deletion", func() {
			var deletedPod *corev1.Pod
			When("a active consumer pod is deleted", func() {
				BeforeEach(func() {
					superStreamName = "active-consumer-delete"
					compositeConsumerName = "active-consumer-delete"
				})
				JustBeforeEach(func() {
					Expect(client.Create(ctx, &compositeConsumer)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: compositeConsumerName, Namespace: "default"},
							&compositeConsumer,
						)

						return compositeConsumer.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))

					var podList corev1.PodList
					err := client.List(ctx, &podList, runtimeclient.InNamespace(compositeConsumer.Namespace), runtimeclient.MatchingLabels(map[string]string{
						managedresource.AnnotationSuperStream: superStreamName,
					}))
					Expect(err).NotTo(HaveOccurred())
					Expect(podList.Items).To(HaveLen(2))
					deletedPod = &podList.Items[0]
					Expect(client.Delete(ctx, deletedPod)).To(Succeed())
				})

				It("ensures a consumer is recreated", func() {
					By("recreating the deleted Pod", func() {
						EventuallyWithOffset(1, func() error {
							var pod corev1.Pod
							err := client.Get(
								ctx,
								types.NamespacedName{Name: deletedPod.Name, Namespace: deletedPod.Namespace},
								&pod,
							)

							return err
						}, 10*time.Second, 1*time.Second).Should(Succeed())
					})
				})
			})
		})
	})
})
