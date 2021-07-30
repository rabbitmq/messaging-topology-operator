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

var _ = Describe("bindingController", func() {
	var binding topology.Binding
	var bindingName string

	JustBeforeEach(func() {
		binding = topology.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: "default",
			},
			Spec: topology.BindingSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	When("creating a binding", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				bindingName = "test-binding-http-error"
				fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
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
				bindingName = "test-binding-go-error"
				fakeRabbitMQClient.DeclareBindingReturns(nil, errors.New("hit a exception"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("hit a exception"),
				})))
			})
		})

		When("the RabbitMQ Client successfully creates a binding", func() {
			BeforeEach(func() {
				bindingName = "test-binding-success"
				fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("sets the status condition to indicate a success in reconciling", func() {
				Expect(client.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})
	})

	When("Deleting a binding", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &binding)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
					&binding,
				)

				return binding.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-http-error"
				fakeRabbitMQClient.DeleteBindingReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &binding)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &topology.Binding{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete binding"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-go-error"
				fakeRabbitMQClient.DeleteBindingReturns(nil, errors.New("some error"))
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &binding)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &topology.Binding{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete binding"))
			})
		})

		When("the RabbitMQ Client successfully deletes a binding", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-success"
				fakeRabbitMQClient.DeleteBindingReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("raises an event to indicate a successful deletion", func() {
				Expect(client.Delete(ctx, &binding)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &topology.Binding{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete binding")),
					ContainElement("Normal SuccessfulDelete successfully deleted binding"),
				))
			})
		})
	})

	Context("finalizer", func() {
		BeforeEach(func() {
			bindingName = "finalizer-test"
		})

		It("sets the correct deletion finalizer to the object", func() {
			Expect(client.Create(ctx, &binding)).To(Succeed())
			Eventually(func() []string {
				var fetched topology.Binding
				Expect(client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &fetched)).To(Succeed())
				return fetched.ObjectMeta.Finalizers
			}, 5).Should(ConsistOf("deletion.finalizers.bindings.rabbitmq.com"))
		})
	})
})
