package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("federation", func() {
	var (
		namespace           = MustHaveEnv("NAMESPACE")
		ctx                 = context.Background()
		federation          = &topology.Federation{}
		federationUri       = "amqp://server-name-my-upstream-test-uri0,amqp://server-name-my-upstream-test-uri1,amqp://server-name-my-upstream-test-uri2"
		federationUriSecret corev1.Secret
	)

	BeforeEach(func() {
		federationUriSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "federation-uri",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"uri": []byte(federationUri),
			},
		}
		Expect(k8sClient.Create(ctx, &federationUriSecret)).To(Succeed())

		federation = &topology.Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "federation",
				Namespace: namespace,
			},
			Spec: topology.FederationSpec{
				Name:       "my-upstream",
				UriSecret:  &corev1.LocalObjectReference{Name: federationUriSecret.Name},
				MessageTTL: 3000,
				Queue:      "a-queue",
				AckMode:    "on-publish",
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &federationUriSecret, &client.DeleteOptions{})).To(Succeed())
	})

	It("works", func() {
		By("federation upstream successfully")
		Expect(k8sClient.Create(ctx, federation, &client.CreateOptions{})).To(Succeed())
		var upstream *rabbithole.FederationUpstream
		Eventually(func() error {
			var err error
			upstream, err = rabbitClient.GetFederationUpstream("/", federation.Spec.Name)
			return err
		}, 30, 2).Should(BeNil())

		Expect(upstream.Name).To(Equal(federation.Spec.Name))
		Expect(upstream.Vhost).To(Equal(federation.Spec.Vhost))
		Expect(upstream.Definition.Uri).To(ConsistOf("amqp://server-name-my-upstream-test-uri0",
			"amqp://server-name-my-upstream-test-uri1",
			"amqp://server-name-my-upstream-test-uri2"))
		Expect(upstream.Definition.Queue).To(Equal(federation.Spec.Queue))
		Expect(upstream.Definition.MessageTTL).To(Equal(int32(federation.Spec.MessageTTL)))
		Expect(upstream.Definition.AckMode).To(Equal(federation.Spec.AckMode))

		By("updating status condition 'Ready'")
		updatedFederation := topology.Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &updatedFederation)).To(Succeed())

		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &updatedFederation)).To(Succeed())
			return updatedFederation.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Federation status condition should be present")

		readyCondition := updatedFederation.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting correct finalizer")
		Expect(updatedFederation.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.federations.rabbitmq.com"))

		By("setting status.observedGeneration")
		Expect(updatedFederation.Status.ObservedGeneration).To(Equal(updatedFederation.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := topology.Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on name, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating federation upstream parameters successfully")
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, federation)).To(Succeed())
		federation.Spec.MessageTTL = 1000
		Expect(k8sClient.Update(ctx, federation, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() int32 {
			var err error
			upstream, err = rabbitClient.GetFederationUpstream("/", federation.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
			return upstream.Definition.MessageTTL
		}, 30, 2).Should(Equal(int32(1000)))

		By("unsetting federation upstream on deletion")
		Expect(k8sClient.Delete(ctx, federation)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetFederationUpstream("/", federation.Spec.Name)
			return err
		}, 10).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})

	When("deletion policy is retain", func() {
		It("deletes k8s resource but keeps the federation in RabbitMQ", func() {
			federationWithRetain := &topology.Federation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "retain-policy-test",
					Namespace: namespace,
				},
				Spec: topology.FederationSpec{
					Name:           "retain-policy-test",
					UriSecret:      &corev1.LocalObjectReference{Name: federationUriSecret.Name},
					DeletionPolicy: "retain",
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
					},
				},
			}

			By("creating a federation with retain policy")
			Expect(k8sClient.Create(ctx, federationWithRetain, &client.CreateOptions{})).To(Succeed())

			By("waiting for the federation to be created in RabbitMQ")
			Eventually(func() error {
				_, err := rabbitClient.GetFederationUpstream("/", federationWithRetain.Spec.Name)
				return err
			}, 30, 2).ShouldNot(HaveOccurred())

			By("deleting the k8s resource")
			Expect(k8sClient.Delete(ctx, federationWithRetain)).To(Succeed())

			By("verifying k8s resource is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: federationWithRetain.Name, Namespace: federationWithRetain.Namespace}, &topology.Federation{})
				return apierrors.IsNotFound(err)
			}, 30, 2).Should(BeTrue())

			By("verifying federation still exists in RabbitMQ")
			_, err := rabbitClient.GetFederationUpstream("/", federationWithRetain.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
