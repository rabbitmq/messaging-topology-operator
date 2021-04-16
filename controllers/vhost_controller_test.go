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

var _ = Describe("vhost-controller", func() {
	var vhost topology.Vhost
	var vhostName string

	JustBeforeEach(func() {
		vhost = topology.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vhostName,
				Namespace: "default",
			},
			Spec: topology.VhostSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				vhostName = "test-http-error"
				fakeRabbitMQClient.PutVhostReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
			})

			It("sets the status condition", func() {
				Expect(client.Create(ctx, &vhost)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
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
				vhostName = "test-go-error"
				fakeRabbitMQClient.PutVhostReturns(nil, errors.New("a go failure"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &vhost)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
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
				vhostName = "test-create-success"
				fakeRabbitMQClient.PutVhostReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("sets the status condition 'Ready' to 'true' ", func() {
				Expect(client.Create(ctx, &vhost)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
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
			fakeRabbitMQClient.PutVhostReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &vhost)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
					&vhost,
				)

				return vhost.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				vhostName = "delete-vhost-http-error"
				fakeRabbitMQClient.DeleteVhostReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &topology.Vhost{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete vhost"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				vhostName = "delete-go-error"
				fakeRabbitMQClient.DeleteVhostReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &topology.Vhost{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete vhost"))
			})
		})

		When("the RabbitMQ cluster is nil", func() {
			BeforeEach(func() {
				vhostName = "delete-client-not-found-error"
			})

			JustBeforeEach(func() {
				prepareNoSuchClusterError()
			})

			It("successfully deletes the vhost regardless", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &topology.Vhost{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())
				Expect(observedEvents()).To(ContainElement("Normal SuccessfulDelete successfully deleted vhost"))
			})
		})

		When("the RabbitMQ Client successfully deletes a vhost", func() {
			BeforeEach(func() {
				vhostName = "delete-vhost-success"
				fakeRabbitMQClient.DeleteVhostReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &topology.Vhost{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())
				observedEvents := observedEvents()
				Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete vhost"))
				Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted vhost"))
			})
		})
	})
})
