package system_tests

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/types"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

var _ = Describe("Users", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		user      *topology.User
	)

	When("relying on the operator to generate a username and password", func() {
		BeforeEach(func() {
			user = &topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user",
					Namespace: namespace,
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
					},
					Tags: []topology.UserTag{"policymaker", "management"},
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
				return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
			}, 30, 2).Should(Succeed())
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
				"Tags":             ConsistOf("policymaker", "management"),
				"HashingAlgorithm": Equal(rabbithole.HashingAlgorithmSHA512),
			}))
			Expect(userInfo.PasswordHash).NotTo(BeEmpty())

			By("creating a client credential set that can be authenticated")
			var err error
			managementEndpoint, err := managementEndpoint(ctx, clientSet, user.Namespace, user.Spec.RabbitmqClusterReference.Name)
			Expect(err).NotTo(HaveOccurred())
			client, err := rabbithole.NewClient(managementEndpoint, rawUsername, rawPassword)
			Expect(err).NotTo(HaveOccurred())
			_, err = client.Overview()
			Expect(err).NotTo(HaveOccurred())

			By("Referencing the location of the Secret in the User's Status")
			generatedUser := &topology.User{}
			Eventually(func() *corev1.LocalObjectReference {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, generatedUser)
				if err != nil {
					return nil
				}

				if generatedUser.Status.Credentials != nil {
					return generatedUser.Status.Credentials
				}

				return nil
			}, 30).ShouldNot(BeNil())
			Expect(generatedUser.Status.Credentials.Name).To(Equal(generatedSecret.Name))

			By("updating status condition 'Ready'")
			updatedUser := topology.User{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &updatedUser)).To(Succeed())

			Eventually(func() []topology.Condition {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &updatedUser)).To(Succeed())
				return updatedUser.Status.Conditions
			}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "User status condition should be present")

			readyCondition := updatedUser.Status.Conditions[0]
			Expect(string(readyCondition.Type)).To(Equal("Ready"))
			Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
			Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

			By("setting correct finalizer")
			Expect(updatedUser.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.users.rabbitmq.com"))

			By("setting status.observedGeneration")
			Expect(updatedUser.Status.ObservedGeneration).To(Equal(updatedUser.GetGeneration()))

			By("not allowing updates on certain fields")
			updateTest := topology.User{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &updateTest)).To(Succeed())
			updateTest.Spec.RabbitmqClusterReference = topology.RabbitmqClusterReference{Name: "a-new-cluster"}
			Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.rabbitmqClusterReference: Forbidden: update on rabbitmqClusterReference is forbidden"))

			By("deleting user")
			Expect(k8sClient.Delete(ctx, user)).To(Succeed())
			Eventually(func() error {
				_, err = rabbitClient.GetUser(rawUsername)
				return err
			}, 30).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Object Not Found"))

			By("deleting the credentials secret")
			Eventually(func() error {
				err := k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
				return err
			}, 30).Should(HaveOccurred())
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
			user = &topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-2",
					Namespace: namespace,
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
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
				return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
			}, 30, 2).Should(Succeed())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8("`got*special_ch$racter5")))
			Expect(generatedSecret.Data).To(HaveKeyWithValue("password", []uint8("-grace.hopper_9453$")))
		})
	})
	When("providing a pre-defined username but autogenerated password", func() {
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
				},
			}
			Expect(k8sClient.Create(ctx, &credentialSecret, &client.CreateOptions{})).To(Succeed())
			user = &topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-3",
					Namespace: namespace,
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
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

		It("sets the value of the Secret according to the provided user", func() {
			By("declaring user")
			Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())

			By("Creating a new Secret with the provided credentials secret")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-3-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
			}, 30, 2).Should(Succeed())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8("`got*special_ch$racter5")))
			Expect(generatedSecret.Data).To(HaveKey("password"))
		})
	})

	When("providing a predefined username & passwordHash", func() {
		const (
			username = "`got*special_ch$racter5"
			password = "S3cur3/P455"
		)
		hash := rabbithole.Base64EncodedSaltedPasswordHashSHA512(password)

		var credentialSecret corev1.Secret
		BeforeEach(func() {
			credentialSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credential-list-secret",
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username":            []byte(username),
					"passwordHash":        []byte(hash),
					"password":            []byte("should$be_ignored"),
					"some.irrelevant.key": []byte("some-useless-value"),
				},
			}
			Expect(k8sClient.Create(ctx, &credentialSecret, &client.CreateOptions{})).To(Succeed())
			user = &topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-4",
					Namespace: namespace,
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
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

		It("declares a user successfully", func() {
			By("declaring user")
			Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())

			By("creating a new Secret with the provided credentials secret")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-4-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
			}, 30, 2).Should(Succeed())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8(username)))
			Expect(generatedSecret.Data).To(HaveKeyWithValue("passwordHash", []uint8(hash)))

			By("ignoring the redundant password")
			Expect(generatedSecret.Data).ToNot(HaveKey("password"))

			By("creating a user that can be authenticated with the original password")
			var err error
			managementEndpoint, err := managementEndpoint(ctx, clientSet, user.Namespace, user.Spec.RabbitmqClusterReference.Name)
			Expect(err).NotTo(HaveOccurred())
			_, err = rabbithole.NewClient(managementEndpoint, username, password)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("providing a predefined username & empty passwordHash", func() {
		const (
			username        = "`got*special_ch$racter5"
			hash            = ""
			ignoredPassword = "should$be_ignored"
		)

		var credentialSecret corev1.Secret
		BeforeEach(func() {
			credentialSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credential-list-secret",
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username":            []byte(username),
					"passwordHash":        []byte(hash),
					"password":            []byte(ignoredPassword),
					"some.irrelevant.key": []byte("some-useless-value"),
				},
			}
			Expect(k8sClient.Create(ctx, &credentialSecret, &client.CreateOptions{})).To(Succeed())
			user = &topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-4",
					Namespace: namespace,
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
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

		It("declares a passwordless user successfully", func() {
			By("declaring user")
			Expect(k8sClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())

			By("creating a new Secret with the provided credentials secret")
			generatedSecretKey := types.NamespacedName{
				Name:      "user-4-user-credentials",
				Namespace: namespace,
			}
			var generatedSecret = &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, generatedSecretKey, generatedSecret)
			}, 30, 2).Should(Succeed())
			Expect(generatedSecret.Data).To(HaveKeyWithValue("username", []uint8(username)))
			Expect(generatedSecret.Data).To(HaveKeyWithValue("passwordHash", []uint8(hash)))

			By("ignoring the redundant password")
			Expect(generatedSecret.Data).ToNot(HaveKey("password"))

			By("creating a user with empty password hash")
			var err error
			user, err := rabbitClient.GetUser(username)
			Expect(err).NotTo(HaveOccurred())
			Expect(user.PasswordHash).To(Equal(""))
		})
	})
})
