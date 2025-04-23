package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("user spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a user with default settings", func() {
		user := User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-user",
				Namespace: namespace,
			},
			Spec: UserSpec{
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		fetcheduser := &User{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      user.Name,
			Namespace: user.Namespace,
		}, fetcheduser)).To(Succeed())
		Expect(fetcheduser.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
			Name: "some-cluster",
		}))
		Expect(len(fetcheduser.Spec.Tags)).To(Equal(0))
	})

	When("creating a user with configuration", func() {
		var tags []UserTag
		var user User
		var username string
		JustBeforeEach(func() {
			user = User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      username,
					Namespace: namespace,
				},
				Spec: UserSpec{
					Tags: tags,
					ImportCredentialsSecret: &corev1.LocalObjectReference{
						Name: "secret-name",
					},
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
				},
			}
		})
		When("creating a user with a list of valid tags", func() {
			BeforeEach(func() {
				tags = []UserTag{"policymaker", "monitoring"}
				username = "valid-user"
			})
			It("successfully creates the user", func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				fetchedUser := &User{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      user.Name,
					Namespace: user.Namespace,
				}, fetchedUser)).To(Succeed())
				Expect(fetchedUser.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
					Name: "some-cluster",
				}))
				Expect(fetchedUser.Spec.ImportCredentialsSecret.Name).To(Equal("secret-name"))
				Expect(fetchedUser.Spec.Tags).To(Equal([]UserTag{"policymaker", "monitoring"}))
			})
		})

		When("creating a user with a list containing an invalid tags", func() {
			BeforeEach(func() {
				tags = []UserTag{"policymaker", "invalid"}
				username = "invalid-user"
			})
			It("fails to create the user", func() {
				Expect(k8sClient.Create(ctx, &user)).To(MatchError(`User.rabbitmq.com "invalid-user" is invalid: spec.tags[1]: Unsupported value: "invalid": supported values: "management", "policymaker", "monitoring", "administrator"`))
			})
		})
	})

	When("creating a user with limits", func() {
		var user User
		var username string
		var userLimits UserLimits
		var connections, channels int32

		JustBeforeEach(func() {
			user = User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      username,
					Namespace: namespace,
				},
				Spec: UserSpec{
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
					UserLimits: &userLimits,
				},
			}
		})

		When("creating a user with valid limits", func() {
			BeforeEach(func() {
				username = "limits-user"
				connections = 5
				channels = 10
				userLimits = UserLimits{
					Connections: &connections,
					Channels:    &channels,
				}
			})
			It("successfully creates the user", func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				fetchedUser := &User{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      user.Name,
					Namespace: user.Namespace,
				}, fetchedUser)).To(Succeed())
				Expect(fetchedUser.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
					Name: "some-cluster",
				}))
				Expect(fetchedUser.Spec.UserLimits).NotTo(BeNil())
				Expect(*fetchedUser.Spec.UserLimits.Connections).To(Equal(connections))
				Expect(*fetchedUser.Spec.UserLimits.Channels).To(Equal(channels))
			})
		})
	})
})
