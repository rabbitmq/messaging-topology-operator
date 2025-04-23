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

var _ = Describe("vhost", func() {

	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		vhost     = &topology.Vhost{}
	)

	BeforeEach(func() {
		vhost = &topology.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace,
			},
			Spec: topology.VhostSpec{
				Name:             "test",
				Tags:             []string{"multi_dc_replication"},
				DefaultQueueType: "stream",
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, vhost)
	})

	It("creates and deletes a vhost successfully", func() {
		By("creating a vhost")
		Expect(k8sClient.Create(ctx, vhost, &client.CreateOptions{})).To(Succeed())
		var fetched *rabbithole.VhostInfo
		Eventually(func() error {
			var err error
			fetched, err = rabbitClient.GetVhost(vhost.Spec.Name)
			return err
		}, 30, 2).ShouldNot(HaveOccurred(), "cannot find created vhost")
		Expect(fetched.Tracing).To(BeFalse())
		Expect(fetched.Tags).To(HaveLen(1))
		Expect(fetched.Tags[0]).To(Equal("multi_dc_replication"))
		Expect(fetched.DefaultQueueType).To(Equal("stream"))

		By("updating status condition 'Ready'")
		updatedVhost := topology.Vhost{}

		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &updatedVhost)).To(Succeed())
			return updatedVhost.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Vhost status condition should be present")

		readyCondition := updatedVhost.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting correct finalizer")
		Expect(updatedVhost.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.vhosts.rabbitmq.com"))

		By("setting status.observedGeneration")
		Expect(updatedVhost.Status.ObservedGeneration).To(Equal(updatedVhost.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := topology.Vhost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Name = "new-name"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.name: Forbidden: updates on name and rabbitmqClusterReference are all forbidden"))

		By("updating vhosts configuration")
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, vhost)).To(Succeed())
		vhost.Spec.Tracing = false
		Expect(k8sClient.Update(ctx, vhost, &client.UpdateOptions{})).To(Succeed())
		Eventually(func() bool {
			var err error
			fetched, err = rabbitClient.GetVhost(vhost.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
			return fetched.Tracing
		}, 30, 2).Should(BeFalse())

		By("deleting a vhost")
		Expect(k8sClient.Delete(ctx, vhost)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetVhost(vhost.Spec.Name)
			return err
		}, 30).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})

	When("deletion policy is retain", func() {
		It("deletes k8s resource but keeps the vhost in RabbitMQ", func() {
			vhostWithRetain := &topology.Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "retain-policy-test",
					Namespace: namespace,
				},
				Spec: topology.VhostSpec{
					Name:           "retain-policy-test",
					DeletionPolicy: "retain",
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
					},
				},
			}

			By("creating a vhost with retain policy")
			Expect(k8sClient.Create(ctx, vhostWithRetain, &client.CreateOptions{})).To(Succeed())

			By("waiting for the vhost to be created in RabbitMQ")
			Eventually(func() error {
				_, err := rabbitClient.GetVhost(vhostWithRetain.Spec.Name)
				return err
			}, 30, 2).ShouldNot(HaveOccurred())

			By("deleting the k8s resource")
			Expect(k8sClient.Delete(ctx, vhostWithRetain)).To(Succeed())

			By("verifying k8s resource is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: vhostWithRetain.Name, Namespace: vhostWithRetain.Namespace}, &topology.Vhost{})
				return apierrors.IsNotFound(err)
			}, 30, 2).Should(BeTrue())

			By("verifying vhost still exists in RabbitMQ")
			_, err := rabbitClient.GetVhost(vhostWithRetain.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("vhost limits are provided", func() {
		var connections, queues int32
		var vhostWithLimits *topology.Vhost

		BeforeEach(func() {
			connections = 108
			queues = 212
			vhostWithLimits = &topology.Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vhost-limits-test",
					Namespace: namespace,
				},
				Spec: topology.VhostSpec{
					Name: "vhost-limits-test",
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
					},
					VhostLimits: &topology.VhostLimits{
						Connections: &connections,
						Queues:      &queues,
					},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(ctx, vhostWithLimits, &client.CreateOptions{})).To(Succeed())
			Eventually(func() error {
				_, err := rabbitClient.GetVhost(vhostWithLimits.Spec.Name)
				return err
			}, 30, 2).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, vhostWithLimits)).To(Succeed())
		})

		It("configures the limits", func() {
			var err error
			var vhostLimitsInfoResponse []rabbithole.VhostLimitsInfo
			Eventually(func() error {
				vhostLimitsInfoResponse, err = rabbitClient.GetVhostLimits(vhostWithLimits.Spec.Name)
				return err
			}, 30, 2).ShouldNot(HaveOccurred())
			Expect(len(vhostLimitsInfoResponse)).To(Equal(1))
			Expect(vhostLimitsInfoResponse[0].Vhost).To(Equal(vhostWithLimits.Spec.Name))
			vhostLimitsValues := vhostLimitsInfoResponse[0].Value
			Expect(len(vhostLimitsValues)).To(Equal(2))
			connectionLimit, ok := vhostLimitsValues["max-connections"]
			Expect(ok).To(BeTrue())
			Expect(connectionLimit).To(Equal(int(connections)))
			queueLimit, ok := vhostLimitsValues["max-queues"]
			Expect(ok).To(BeTrue())
			Expect(queueLimit).To(Equal(int(queues)))
		})
	})
})
