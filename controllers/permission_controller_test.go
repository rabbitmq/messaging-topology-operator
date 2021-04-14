package controllers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("permission-controller", func() {
	var permission topology.Permission
	var permissionName string

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
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				permissionName = "test-http-error"
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
				permissionName = "test-go-error"
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
				permissionName = "test-create-success"
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
				permissionName = "delete-permission-http-error"
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
				permissionName = "delete-go-error"
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

		When("the RabbitMQ cluster is nil", func() {
			BeforeEach(func() {
				permissionName = "delete-client-not-found-error"
			})

			JustBeforeEach(func() {
				prepareNoSuchClusterError()
			})

			It("successfully deletes the permission regardless", func() {
				Expect(client.Delete(ctx, &permission)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())
				Expect(observedEvents()).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
			})
		})

		When("the RabbitMQ Client successfully deletes a permission", func() {
			BeforeEach(func() {
				permissionName = "delete-permission-success"
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
})
