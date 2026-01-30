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
	. "github.com/onsi/gomega/gstruct"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

var _ = Describe("Topic Permission", func() {
	var (
		namespace       = MustHaveEnv("NAMESPACE")
		ctx             = context.Background()
		topicPermission *topology.TopicPermission
		user            *topology.User
		exchange        *topology.Exchange
		username        string
	)

	BeforeEach(func() {
		user = &topology.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "userabc",
				Namespace: namespace,
			},
			Spec: topology.UserSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Tags: []topology.UserTag{"management"},
			},
		}
		Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())
		generatedSecretKey := types.NamespacedName{
			Name:      "userabc-user-credentials",
			Namespace: namespace,
		}
		var generatedSecret = &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
		}, 30, 2).Should(Succeed())
		username = string(generatedSecret.Data["username"])

		exchange = &topology.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchangeabc",
				Namespace: namespace,
			},
			Spec: topology.ExchangeSpec{
				Name: "exchangeabc",
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, exchange, &client.CreateOptions{})).To(Succeed())

		topicPermission = &topology.TopicPermission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-topic-perm",
				Namespace: namespace,
			},
			Spec: topology.TopicPermissionSpec{
				Vhost: "/",
				User:  username,
				Permissions: topology.TopicPermissionConfig{
					Exchange: exchange.Spec.Name,
					Read:     ".*",
					Write:    ".*",
				},
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, user)).To(Succeed())
		Eventually(func() string {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{}); err != nil {
				return err.Error()
			}
			return ""
		}, 10).Should(ContainSubstring("not found"))
		Expect(k8sClient.Delete(ctx, exchange)).To(Succeed())
		Eventually(func() string {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &topology.Exchange{}); err != nil {
				return err.Error()
			}
			return ""
		}, 10).Should(ContainSubstring("not found"))
	})

	DescribeTable("Server configurations updates", func(testcase string) {
		if testcase == "UserReference" {
			topicPermission.Spec.User = ""
			topicPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: user.Name}
		}
		Expect(k8sClient.Create(ctx, topicPermission, &client.CreateOptions{})).To(Succeed())
		var fetchedPermissionInfo []rabbithole.TopicPermissionInfo
		Eventually(func() error {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetTopicPermissionsIn(topicPermission.Spec.Vhost, username)
			return err
		}, 30, 2).Should(Not(HaveOccurred()))
		Expect(fetchedPermissionInfo).To(HaveLen(1))
		Expect(fetchedPermissionInfo[0]).To(
			MatchFields(IgnoreExtras, Fields{
				"Vhost":    Equal(topicPermission.Spec.Vhost),
				"User":     Equal(username),
				"Exchange": Equal(topicPermission.Spec.Permissions.Exchange),
				"Read":     Equal(topicPermission.Spec.Permissions.Read),
				"Write":    Equal(topicPermission.Spec.Permissions.Write)}))

		By("updating status condition 'Ready'")
		updated := topology.TopicPermission{}

		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: topicPermission.Name, Namespace: topicPermission.Namespace}, &updated)).To(Succeed())
			return updated.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "status condition should be present")

		readyCondition := updated.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting correct finalizer")
		Expect(updated.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.topicpermissions.rabbitmq.com"))

		By("setting status.observedGeneration")
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := topology.TopicPermission{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: topicPermission.Name, Namespace: topicPermission.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on exchange, user, userReference, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating topic permissions successfully")
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: topicPermission.Name, Namespace: topicPermission.Namespace}, topicPermission)).To(Succeed())
		topicPermission.Spec.Permissions.Write = "^$"
		topicPermission.Spec.Permissions.Read = "^$"
		Expect(k8sClient.Update(ctx, topicPermission, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() string {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetTopicPermissionsIn(topicPermission.Spec.Vhost, username)
			Expect(err).NotTo(HaveOccurred())
			return fetchedPermissionInfo[0].Write
		}, 20, 2).Should(Equal("^$"))
		Expect(fetchedPermissionInfo).To(HaveLen(1))
		Expect(fetchedPermissionInfo[0]).To(
			MatchFields(IgnoreExtras, Fields{
				"Vhost":    Equal(topicPermission.Spec.Vhost),
				"User":     Equal(username),
				"Exchange": Equal(topicPermission.Spec.Permissions.Exchange),
				"Read":     Equal("^$"),
				"Write":    Equal("^$")}))

		By("clearing permissions successfully")
		Expect(k8sClient.Delete(ctx, topicPermission)).To(Succeed())
		Eventually(func() int {
			permList, err := rabbitClient.ListTopicPermissionsOf(username)
			Expect(err).NotTo(HaveOccurred())
			return len(permList)
		}, 10, 2).Should(Equal(0))
	},

		Entry("manage topic permissions successfully when spec.user is set", "User"),
		Entry("manage topic permissions successfully when spec.userReference is set", "UserReference"),
	)
})
