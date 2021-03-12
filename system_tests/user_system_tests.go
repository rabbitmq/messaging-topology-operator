package system_tests

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/types"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

var _ = Describe("Users", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		user      *topologyv1alpha1.User
	)

	When("relying on the operator to generate a username and password", func() {
		BeforeEach(func() {
			user = &topologyv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user",
					Namespace: namespace,
				},
				Spec: topologyv1alpha1.UserSpec{
					RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
						Name:      rmq.Name,
						Namespace: rmq.Namespace,
					},
					Tags: []topologyv1alpha1.UserTag{"policymaker", "management"},
				},
			}
		})

		It("declares and deletes a user successfully", func() {
			By("declaring user")
			Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())

			By("Creating a Secret with the generated credentials")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Eventually(func() error {
				var err error
				err = k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
				return err
			}, 30, 2).Should(BeNil())
			Expect(generatedSecret.Data).To(HaveKey("username"))
			Expect(generatedSecret.Data).To(HaveKey("password"))

			rawUsername := string(generatedSecret.Data["username"])
			rawPassword := string(generatedSecret.Data["password"])

			By("setting the correct user info")
			var userInfo *rabbithole.UserInfo
			Eventually(func() error {
				var err error
				userInfo, err = rabbitClient.GetUser(rawUsername)
				return err
			}, 10, 2).Should(BeNil())

			Expect(*userInfo).To(MatchFields(IgnoreExtras, Fields{
				"Name":             Equal(rawUsername),
				"Tags":             Equal("policymaker,management"),
				"HashingAlgorithm": Equal(rabbithole.HashingAlgorithmSHA512),
			}))
			Expect(userInfo.PasswordHash).NotTo(BeEmpty())

			By("creating a client credential set that can be authenticated")
			var err error
			managementEndpoint, err := managementEndpoint(ctx, clientSet, &user.Spec.RabbitmqClusterReference)
			Expect(err).NotTo(HaveOccurred())
			client, err := rabbithole.NewClient(managementEndpoint, rawUsername, rawPassword)
			Expect(err).NotTo(HaveOccurred())
			_, err = client.Overview()
			Expect(err).NotTo(HaveOccurred())

			By("Referencing the location of the Secret in the User's Status")
			generatedUser := &topologyv1alpha1.User{}
			Eventually(func() *corev1.LocalObjectReference {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, generatedUser)
				if err != nil {
					return nil
				}

				if generatedUser.Status.Credentials != nil {
					return generatedUser.Status.Credentials
				}

				return nil
			}, 5).ShouldNot(BeNil())
			Expect(generatedUser.Status.Credentials.Name).To(Equal(generatedSecret.Name))

			By("deleting user")
			Expect(k8sClient.Delete(ctx, user)).To(Succeed())
			Eventually(func() error {
				_, err = rabbitClient.GetUser(rawUsername)
				return err
			}, 5).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Object Not Found"))

			By("deleting the credentials secret")
			Eventually(func() error {
				err := k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
				return err
			}, 5).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Object Not Found"))
		})
	})

	When("providing a pre-defined username & password", func() {
		var credentialSecret corev1.Secret
		BeforeEach(func() {
			credentialSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credential-list-secret",
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"some.irrelevant.key": []byte("some-useless-value"),
					"username":            []byte("`got*special_ch$racter5"),
					"password":            []byte("-grace.hopper_9453$"),
				},
			}
			Expect(k8sClient.Create(ctx, &credentialSecret, &client.CreateOptions{})).To(Succeed())
			user = &topologyv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-2",
					Namespace: namespace,
				},
				Spec: topologyv1alpha1.UserSpec{
					RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
						Name:      rmq.Name,
						Namespace: rmq.Namespace,
					},
					ImportCredentialsSecret: &corev1.LocalObjectReference{
						Name: credentialSecret.Name,
					},
				},
			}
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), &credentialSecret)).ToNot(HaveOccurred())
			Expect(k8sClient.Delete(context.Background(), user)).To(Succeed())
		})

		It("sets the value of the Secret according to the provided credentials", func() {
			By("declaring user")
			Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())

			By("Creating a new Secret with the provided credentials secret")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-2-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Eventually(func() error {
				var err error
				err = k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
				return err
			}, 30, 2).Should(BeNil())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8("`got*special_ch$racter5")))
			Expect(generatedSecret.Data).To(HaveKeyWithValue("password", []uint8("-grace.hopper_9453$")))
		})
	})
})
