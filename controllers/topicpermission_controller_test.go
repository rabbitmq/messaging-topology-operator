package controllers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("topicpermission-controller", func() {
	var topicperm topology.TopicPermission
	var user topology.User
	var name string
	var userName string

	When("validating RabbitMQ Client failures with username", func() {
		JustBeforeEach(func() {
			topicperm = topology.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: topology.TopicPermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
					User:  "example",
					Vhost: "example",
				},
			}
		})

		Context("creation", func() {
			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					name = "test-with-username-http-error"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("a failure"))
				})

				It("sets the status condition", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a failure"),
					})))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					name = "test-with-username-go-error"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(nil, errors.New("a go failure"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a go failure"),
					})))
				})
			})
		})

		Context("deletion", func() {
			JustBeforeEach(func() {
				fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(client.Create(ctx, &topicperm)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
						&topicperm,
					)

					return topicperm.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					name = "delete-with-username-topicperm-http-error"
					fakeRabbitMQClient.DeleteTopicPermissionsInReturns(&http.Response{
						Status:     "502 Bad Gateway",
						StatusCode: http.StatusBadGateway,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete topicpermission"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					name = "delete-with-username-go-error"
					fakeRabbitMQClient.DeleteTopicPermissionsInReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete topicpermission"))
				})
			})
		})
	})

	When("validating RabbitMQ Client failures with userRef", func() {
		JustBeforeEach(func() {
			user = topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: "default",
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			topicperm = topology.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: topology.TopicPermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					UserReference: &corev1.LocalObjectReference{
						Name: userName,
					},
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.DeleteTopicPermissionsInReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.DeleteUserReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
		})

		Context("creation", func() {
			When("user not exist", func() {
				BeforeEach(func() {
					name = "test-with-userref-create-not-exist"
					userName = "topic-perm-example-create-not-exist"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Message": Equal("failed create Permission, missing User"),
						"Status":  Equal(corev1.ConditionFalse),
					})))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					name = "test-with-userref-create-success"
					userName = "topic-perm-example-create-success"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &user)).To(Succeed())
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
				})
			})
		})

		Context("deletion", func() {
			JustBeforeEach(func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				Expect(client.Create(ctx, &topicperm)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
						&topicperm,
					)

					return topicperm.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("Secret User is removed first", func() {
				BeforeEach(func() {
					name = "test-with-userref-delete-secret"
					userName = "topic-perm-example-delete-secret-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      user.Name + "-user-credentials",
							Namespace: user.Namespace,
						},
					})).To(Succeed())
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})

			When("User is removed first", func() {
				BeforeEach(func() {
					name = "test-with-userref-delete-user"
					userName = "topic-perm-example-delete-user-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &user)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					name = "test-with-userref-delete-success"
					userName = "topic-perm-example-delete-success"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})
		})

		Context("ownerref", func() {
			BeforeEach(func() {
				name = "ownerref-with-userref-test"
				userName = "topic-perm-topic-perm-user"
			})

			It("sets the correct deletion ownerref to the object", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				Expect(client.Create(ctx, &topicperm)).To(Succeed())
				Eventually(func() []metav1.OwnerReference {
					var fetched topology.TopicPermission
					err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &fetched)
					if err != nil {
						return []metav1.OwnerReference{}
					}
					return fetched.ObjectMeta.OwnerReferences
				}, 5).Should(Not(BeEmpty()))
			})
		})
	})
})
