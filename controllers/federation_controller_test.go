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
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("federation-controller", func() {
	var federation topology.Federation
	var name string

	JustBeforeEach(func() {
		federation = topology.Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: topology.FederationSpec{
				Name:      "my-federation-upstream",
				Vhost:     "/test",
				UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"},
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	When("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				name = "test-federation-http-error"
				fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &federation)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("some HTTP error"),
				})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				name = "test-federation-go-error"
				fakeRabbitMQClient.PutFederationUpstreamReturns(nil, errors.New("some go failure here"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &federation)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("some go failure here"),
				})))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				name = "test-federation-success"
				fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("sets the status condition 'Ready' to 'true'", func() {
				Expect(client.Create(ctx, &federation)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})
	})

	When("deletion", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &federation)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
					&federation,
				)

				return federation.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				name = "delete-federation-http-error"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &federation)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &topology.Federation{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete federation upstream parameter"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				name = "delete-federation-go-error"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &federation)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &topology.Federation{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete federation upstream parameter"))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				name = "delete-federation-success"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &federation)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &topology.Federation{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())
				observedEvents := observedEvents()
				Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete federation upstream parameter"))
				Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted federation upstream parameter"))
			})
		})
	})
})
