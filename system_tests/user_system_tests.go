package system_tests

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

var _ = Describe("Users", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		user      *topologyv1alpha1.User
	)

	BeforeEach(func() {
		user = &topologyv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.UserSpec{
				RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
				Name: "user-test",
				Tags: []topologyv1alpha1.UserTag{"policymaker", "management"},
			},
		}
	})

	It("declares and deletes a user successfully", func() {
		By("declaring user")
		Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())
		var userInfo *rabbithole.UserInfo
		Eventually(func() error {
			var err error
			userInfo, err = rabbitClient.GetUser(user.Spec.Name)
			return err
		}, 10, 2).Should(BeNil())

		Expect(*userInfo).To(MatchFields(IgnoreExtras, Fields{
			"Name":             Equal(user.Spec.Name),
			"Tags":             Equal("policymaker,management"),
			"HashingAlgorithm": Equal(rabbithole.HashingAlgorithmSHA512),
		}))
		Expect(userInfo.PasswordHash).NotTo(BeEmpty())

		By("deleting user")
		Expect(k8sClient.Delete(ctx, user)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetUser(user.Spec.Name)
			return err
		}, 5).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
