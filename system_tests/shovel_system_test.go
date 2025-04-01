package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Shovel", func() {
	var (
		namespace    = MustHaveEnv("NAMESPACE")
		ctx          = context.Background()
		shovel       = &topology.Shovel{}
		srcUri       = "amqp://server-test-src0,amqp://server-test-src1"
		destUri      = "amqp://server-test-dest0,amqp://server-test-dest1"
		shovelSecret corev1.Secret
	)

	BeforeEach(func() {
		shovelSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shovel-uri",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"srcUri":  []byte(srcUri),
				"destUri": []byte(destUri),
			},
		}
		Expect(k8sClient.Create(ctx, &shovelSecret)).To(Succeed())

		shovel = &topology.Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: topology.ShovelSpec{
				UriSecret:         &corev1.LocalObjectReference{Name: shovelSecret.Name},
				SourceDeleteAfter: "never",
				AckMode:           "no-ack",
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &shovelSecret, &client.DeleteOptions{})).To(Succeed())
	})

	It("works", func() {
		shovel.Name = "shovel"
		shovel.Spec.Name = "my-upstream"
		shovel.Spec.SourceQueue = "a-queue"
		shovel.Spec.SourceConsumerArgs = &runtime.RawExtension{Raw: []byte(`{"x-priority": 5}`)}
		shovel.Spec.DestinationQueue = "another-queue"
		shovel.Spec.DestinationPublishProperties = &runtime.RawExtension{Raw: []byte(`{"delivery_mode": 2}`)}

		By("declaring shovel successfully")
		shovelInfo := declareAssertShovelCommonProperties(ctx, shovel)

		Expect(shovelInfo.Definition.DestinationQueue).To(Equal(shovel.Spec.DestinationQueue))
		Expect(shovelInfo.Definition.SourceQueue).To(Equal(shovel.Spec.SourceQueue))
		Expect(shovelInfo.Definition.SourceConsumerArgs).To(HaveKeyWithValue("x-priority", float64(5)))
		Expect(shovelInfo.Definition.DestinationPublishProperties).To(HaveKeyWithValue("delivery_mode", float64(2)))

		By("updating status condition 'Ready'")
		updatedShovel := topology.Shovel{}

		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &updatedShovel)).To(Succeed())
			return updatedShovel.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Shovel status condition should be present")

		readyCondition := updatedShovel.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting correct finalizer")
		Expect(updatedShovel.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.shovels.rabbitmq.com"))

		By("setting status.observedGeneration")
		Expect(updatedShovel.Status.ObservedGeneration).To(Equal(updatedShovel.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := topology.Shovel{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Name = "a-new-shovel"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.name: Forbidden: updates on name, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating shovel parameters successfully")
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, shovel)).To(Succeed())
		shovel.Spec.PrefetchCount = 200
		Expect(k8sClient.Update(ctx, shovel, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() int {
			info, err := rabbitClient.GetShovel("/", shovel.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
			return info.Definition.PrefetchCount
		}, 30, 2).Should(Equal(200))

		By("deleting shovel configuration on deletion")
		Expect(k8sClient.Delete(ctx, shovel)).To(Succeed())
		assertShovelDeleted(shovel)
	})

	It("works with a shovel using amqp10 protocol", func() {
		shovel.Name = "shovel-amqp10"
		shovel.Spec.Name = "my-upstream-amqp10"
		shovel.Spec.SourceProtocol = "amqp10"
		shovel.Spec.DestinationProtocol = "amqp10"
		shovel.Spec.SourceAddress = "/an-exchange"
		shovel.Spec.DestinationAddress = "/a-queue"
		shovel.Spec.DestinationApplicationProperties = &runtime.RawExtension{Raw: []byte(`{"a-key": "a-value"}`)}
		shovel.Spec.DestinationMessageAnnotations = &runtime.RawExtension{Raw: []byte(`{"a-key": "a-annotation"}`)}
		shovel.Spec.DestinationProperties = &runtime.RawExtension{Raw: []byte(`{"content_type": "text/plain"}`)}
		shovel.Spec.DestinationAddForwardHeaders = true
		shovel.Spec.DestinationAddTimestampHeader = true

		By("declaring shovel successfully")
		shovelInfo := declareAssertShovelCommonProperties(ctx, shovel)

		Expect(shovelInfo.Definition.SourceProtocol).To(Equal("amqp10"))
		Expect(shovelInfo.Definition.DestinationProtocol).To(Equal("amqp10"))
		Expect(shovelInfo.Definition.SourceAddress).To(Equal("/an-exchange"))
		Expect(shovelInfo.Definition.DestinationAddress).To(Equal("/a-queue"))
		Expect(shovelInfo.Definition.DestinationApplicationProperties).To(HaveKeyWithValue("a-key", "a-value"))
		Expect(shovelInfo.Definition.DestinationMessageAnnotations).To(HaveKeyWithValue("a-key", "a-annotation"))
		Expect(shovelInfo.Definition.DestinationProperties).To(HaveKeyWithValue("content_type", "text/plain"))
		Expect(shovelInfo.Definition.DestinationAddForwardHeaders).To(BeTrue())
		Expect(shovelInfo.Definition.DestinationAddTimestampHeader).To(BeTrue())

		By("deleting shovel configuration on deletion")
		Expect(k8sClient.Delete(ctx, shovel)).To(Succeed())
		assertShovelDeleted(shovel)
	})

	When("deletion policy is retain", func() {
		It("deletes k8s resource but keeps the shovel in RabbitMQ", func() {
			shovelWithRetain := &topology.Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "retain-policy-test",
					Namespace: namespace,
				},
				Spec: topology.ShovelSpec{
					Name:           "retain-policy-test",
					UriSecret:      &corev1.LocalObjectReference{Name: shovelSecret.Name},
					DeletionPolicy: "retain",
					SourceQueue:    "test-queue",
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
					},
				},
			}

			By("creating a shovel with retain policy")
			Expect(k8sClient.Create(ctx, shovelWithRetain, &client.CreateOptions{})).To(Succeed())

			By("waiting for the shovel to be created in RabbitMQ")
			Eventually(func() error {
				_, err := rabbitClient.GetShovel("/", shovelWithRetain.Spec.Name)
				return err
			}, 30, 2).ShouldNot(HaveOccurred())

			By("deleting the k8s resource")
			Expect(k8sClient.Delete(ctx, shovelWithRetain)).To(Succeed())

			By("verifying k8s resource is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: shovelWithRetain.Name, Namespace: shovelWithRetain.Namespace}, &topology.Shovel{})
				return apierrors.IsNotFound(err)
			}, 30, 2).Should(BeTrue())

			By("verifying shovel still exists in RabbitMQ")
			_, err := rabbitClient.GetShovel("/", shovelWithRetain.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func declareAssertShovelCommonProperties(ctx context.Context, shovel *topology.Shovel) *rabbithole.ShovelInfo {
	Expect(k8sClient.Create(ctx, shovel, &client.CreateOptions{})).To(Succeed())
	var shovelInfo *rabbithole.ShovelInfo
	Eventually(func() error {
		var err error
		shovelInfo, err = rabbitClient.GetShovel("/", shovel.Spec.Name)
		return err
	}, 30, 2).Should(BeNil())

	Expect(shovelInfo.Name).To(Equal(shovel.Spec.Name))
	Expect(shovelInfo.Vhost).To(Equal(shovel.Spec.Vhost))
	Expect(shovelInfo.Definition.SourceURI).To(
		ConsistOf("amqp://server-test-src0",
			"amqp://server-test-src1"))
	Expect(shovelInfo.Definition.DestinationURI).To(
		ConsistOf("amqp://server-test-dest0",
			"amqp://server-test-dest1"))
	Expect(shovelInfo.Definition.AckMode).To(Equal(shovel.Spec.AckMode))
	Expect(string(shovelInfo.Definition.SourceDeleteAfter)).To(Equal(shovel.Spec.SourceDeleteAfter))
	return shovelInfo
}

func assertShovelDeleted(shovel *topology.Shovel) {
	var err error
	Eventually(func() error {
		_, err = rabbitClient.GetShovel("/", shovel.Spec.Name)
		return err
	}, 10).Should(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Object Not Found"))
}
