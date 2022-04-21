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

var _ = Describe("permission-controller", func() {
	var permission topology.Permission
	var user topology.User
	var permissionName string
	var userName string

	When("validating RabbitMQ Client failures with username", func() {
		JustBeforeEach(func() {
			permission = topology.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "default",
				},
				Spec: topology.PermissionSpec{
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
					permissionName = "test-with-username-http-error"
					fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("a failure"))
				})

				It("sets the status condition", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a failure"),
					})))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					permissionName = "test-with-username-go-error"
					fakeRabbitMQClient.UpdatePermissionsInReturns(nil, errors.New("a go failure"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a go failure"),
					})))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					permissionName = "test-with-username-create-success"
					fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
				})
			})
		})

		Context("deletion", func() {
			JustBeforeEach(func() {
				fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(client.Create(ctx, &permission)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
						&permission,
					)

					return permission.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-permission-http-error"
					fakeRabbitMQClient.ClearPermissionsInReturns(&http.Response{
						Status:     "502 Bad Gateway",
						StatusCode: http.StatusBadGateway,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete permission"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-go-error"
					fakeRabbitMQClient.ClearPermissionsInReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete permission"))
				})
			})

			When("the RabbitMQ Client successfully deletes a permission", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-permission-success"
					fakeRabbitMQClient.ClearPermissionsInReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
				})

				It("publishes a normal event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})
		})

		Context("finalizer", func() {
			BeforeEach(func() {
				permissionName = "finalizer-with-username-test"
			})

			It("sets the correct deletion finalizer to the object", func() {
				Expect(client.Create(ctx, &permission)).To(Succeed())
				Eventually(func() []string {
					var fetched topology.Permission
					err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &fetched)
					if err != nil {
						return []string{}
					}
					return fetched.ObjectMeta.Finalizers
				}, 5).Should(ConsistOf("deletion.finalizers.permissions.rabbitmq.com"))
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
			permission = topology.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "default",
				},
				Spec: topology.PermissionSpec{
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
			fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.ClearPermissionsInReturns(&http.Response{
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
					permissionName = "test-with-userref-create-not-exist"
					userName = "example-create-not-exist"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Message": Equal("failed create Permission, missing User"),
						"Status":  Equal(corev1.ConditionFalse),
					})))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-create-success"
					userName = "example-create-success"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &user)).To(Succeed())
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
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
				Expect(client.Create(ctx, &permission)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
						&permission,
					)

					return permission.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("Secret User is removed first", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-secret"
					userName = "example-delete-secret-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      user.Name + "-user-credentials",
							Namespace: user.Namespace,
						},
					})).To(Succeed())
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})

			When("User is removed first", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-user"
					userName = "example-delete-user-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &user)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Warning UserNotExist user already removed; no need to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-success"
					userName = "example-delete-success"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})
		})

		Context("ownerref", func() {
			BeforeEach(func() {
				permissionName = "ownerref-with-userref-test"
				userName = "example-ownerref"
			})

			It("sets the correct deletion ownerref to the object", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				Expect(client.Create(ctx, &permission)).To(Succeed())
				Eventually(func() []metav1.OwnerReference {
					var fetched topology.Permission
					err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &fetched)
					if err != nil {
						return []metav1.OwnerReference{}
					}
					return fetched.ObjectMeta.OwnerReferences
				}, 5).Should(Not(BeEmpty()))
			})
		})
	})

	When("a permission references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			permissionName = "test-permission-prohibited"
			permission = topology.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "prohibited",
				},
				Spec: topology.PermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &permission)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
					&permission,
				)

				return permission.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(topology.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a permission references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			permissionName = "test-permission-allowed"
			permission = topology.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "allowed",
				},
				Spec: topology.PermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &permission)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
					&permission,
				)

				return permission.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a permission references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			permissionName = "test-permission-allowed-when-allow-all"
			permission = topology.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "prohibited",
				},
				Spec: topology.PermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &permission)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
					&permission,
				)

				return permission.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
