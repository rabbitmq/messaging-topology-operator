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

	When("relying on the operator to generate a password", func() {
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

			By("Creating a Secret with the generated credentials")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-test-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, generatedSecretKey, generatedSecret)).To(Succeed())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8(user.Spec.Name)))
			Expect(generatedSecret.Data).To(HaveKey("password"))

			By("creating a client credential set that can be authenticated")
			var err error
			rawUsername := string(generatedSecret.Data["username"])
			rawPassword := string(generatedSecret.Data["password"])
			Eventually(func() string {
				output, _ := kubectl(
					"-n",
					rmq.Namespace,
					"exec",
					"svc/"+rmq.Name,
					"--",
					"rabbitmqctl",
					`authenticate_user`,
					rawUsername,
					rawPassword,
				)
				return string(output)
			}, 5).Should(ContainSubstring("Success"))

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

			// TODO: Support update workflow
			// By("updating the user tags and preserving the user's credentials")
			// user.Spec.Tags = []topologyv1alpha1.UserTag{}
			// Expect(k8sClient.Update(ctx, user, &client.UpdateOptions{})).To(Succeed())
			// var updatedUserInfo *rabbithole.UserInfo
			// Eventually(func() error {
			// 	var err error
			// 	updatedUserInfo, err = rabbitClient.GetUser(user.Spec.Name)
			// 	return err
			// }, 10, 2).Should(BeNil())

			// Expect(*updatedUserInfo).To(MatchFields(IgnoreExtras, Fields{
			// 	"Name":             Equal(user.Spec.Name),
			// 	"Tags":             Equal(""),
			// 	"HashingAlgorithm": Equal(rabbithole.HashingAlgorithmSHA512),
			// }))
			// Expect(updatedUserInfo.PasswordHash).To(Equal(userInfo.PasswordHash))

			By("deleting user")
			Expect(k8sClient.Delete(ctx, user)).To(Succeed())
			Eventually(func() error {
				_, err = rabbitClient.GetUser(user.Spec.Name)
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

	When("providing a pre-defined password", func() {
		var passwordSecret corev1.Secret
		BeforeEach(func() {
			passwordSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credential-list-secret",
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"some.irrelevant.key": []byte("some-useless-value"),
					"my-password":         []byte("-grace.hopper_9453$"),
				},
			}
			Expect(k8sClient.Create(ctx, &passwordSecret, &client.CreateOptions{})).To(Succeed())
			user = &topologyv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-test-2",
					Namespace: namespace,
				},
				Spec: topologyv1alpha1.UserSpec{
					RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
						Name:      rmq.Name,
						Namespace: rmq.Namespace,
					},
					Name: "user-test-2",
					Tags: []topologyv1alpha1.UserTag{"policymaker", "management"},
					ImportPasswordSecret: topologyv1alpha1.ImportPasswordSecret{
						Name: &corev1.LocalObjectReference{
							Name: passwordSecret.ObjectMeta.Name,
						},
						PasswordKey: "my-password",
					},
				},
			}
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), &passwordSecret)).ToNot(HaveOccurred())
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

			By("Creating a Secret with the generated credentials")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-test-2-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, generatedSecretKey, generatedSecret)).To(Succeed())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8(user.Spec.Name)))
			Expect(generatedSecret.Data).To(HaveKeyWithValue("password", []uint8(passwordSecret.Data["my-password"])))

			By("creating a client credential set that can be authenticated")
			var err error
			rawUsername := string(generatedSecret.Data["username"])
			rawPassword := string(generatedSecret.Data["password"])
			Eventually(func() string {
				output, _ := kubectl(
					"-n",
					rmq.Namespace,
					"exec",
					"svc/"+rmq.Name,
					"--",
					"rabbitmqctl",
					`authenticate_user`,
					rawUsername,
					rawPassword,
				)
				return string(output)
			}, 5).Should(ContainSubstring("Success"))

			By("deleting user")
			Expect(k8sClient.Delete(ctx, user)).To(Succeed())
			Eventually(func() error {
				_, err = rabbitClient.GetUser(user.Spec.Name)
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
})
