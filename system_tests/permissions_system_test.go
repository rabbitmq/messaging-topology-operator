package system_tests

import (
	"context"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

var _ = Describe("Permission", func() {
	var (
		namespace  = MustHaveEnv("NAMESPACE")
		ctx        = context.Background()
		permission *topology.Permission
		user       *topology.User
		username   string
	)

	BeforeEach(func() {
		user = &topology.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testuser",
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
			Name:      "testuser-user-credentials",
			Namespace: namespace,
		}
		var generatedSecret = &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
		}, 30, 2).Should(Succeed())
		username = string(generatedSecret.Data["username"])

		permission = &topology.Permission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-permission",
				Namespace: namespace,
			},
			Spec: topology.PermissionSpec{
				Vhost: "/",
				User:  username,
				Permissions: topology.VhostPermissions{
					Configure: ".*",
					Read:      ".*",
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
	})

	DescribeTable("Server configurations updates", func(testcase string) {
		if testcase == "UserReference" {
			permission.Spec.User = ""
			permission.Spec.UserReference = &corev1.LocalObjectReference{Name: user.Name}
		}
		Expect(k8sClient.Create(ctx, permission, &client.CreateOptions{})).To(Succeed())
		var fetchedPermissionInfo rabbithole.PermissionInfo
		Eventually(func() error {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetPermissionsIn(permission.Spec.Vhost, username)
			return err
		}, 20, 2).Should(Not(HaveOccurred()))
		Expect(fetchedPermissionInfo).To(MatchFields(IgnoreExtras, Fields{
			"Vhost":     Equal(permission.Spec.Vhost),
			"User":      Equal(username),
			"Configure": Equal(permission.Spec.Permissions.Configure),
			"Read":      Equal(permission.Spec.Permissions.Read),
			"Write":     Equal(permission.Spec.Permissions.Write),
		}))

		By("updating status condition 'Ready'")
		updatedPermission := topology.Permission{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &updatedPermission)).To(Succeed())

		Expect(updatedPermission.Status.Conditions).To(HaveLen(1))
		readyCondition := updatedPermission.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedPermission.Status.ObservedGeneration).To(Equal(updatedPermission.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := topology.Permission{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating permissions successfully")
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, permission)).To(Succeed())
		permission.Spec.Permissions.Write = ".*"
		permission.Spec.Permissions.Read = "^$"
		Expect(k8sClient.Update(ctx, permission, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() string {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetPermissionsIn(permission.Spec.Vhost, username)
			Expect(err).NotTo(HaveOccurred())
			return fetchedPermissionInfo.Write
		}, 20, 2).Should(Equal(".*"))
		Expect(fetchedPermissionInfo).To(MatchFields(IgnoreExtras, Fields{
			"Vhost":     Equal(permission.Spec.Vhost),
			"User":      Equal(username),
			"Configure": Equal(permission.Spec.Permissions.Configure),
			"Read":      Equal("^$"),
			"Write":     Equal(".*"),
		}))

		By("revoking permissions successfully")
		Expect(k8sClient.Delete(ctx, permission)).To(Succeed())
		Eventually(func() int {
			permissionInfos, err := rabbitClient.ListPermissionsOf(username)
			Expect(err).NotTo(HaveOccurred())
			return len(permissionInfos)
		}, 10, 2).Should(Equal(0))
	},

		Entry("grants and revokes permissions successfully when spec.user is set", "User"),
		Entry("grants and revokes permissions successfully when spec.userReference is set", "UserReference"),
	)
})
