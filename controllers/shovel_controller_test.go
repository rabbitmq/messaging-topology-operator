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

var _ = Describe("shovel-controller", func() {
	var shovel topology.Shovel
	var shovelName string

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
			shovel = topology.Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shovelName,
					Namespace: "default",
				},
				Spec: topology.ShovelSpec{
					Name:      "my-shovel-configuration",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "shovel-uri-secret"},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
				},
			}
		})

		When("creation", func() {
			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					shovelName = "test-shovel-http-error"
					fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("some HTTP error"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &shovel)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
							&shovel,
						)

						return shovel.Status.Conditions
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
					shovelName = "test-shovel-go-error"
					fakeRabbitMQClient.DeclareShovelReturns(nil, errors.New("a go failure"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &shovel)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
							&shovel,
						)

						return shovel.Status.Conditions
					}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a go failure"),
					})))
				})
			})

			Context("success", func() {
				BeforeEach(func() {
					shovelName = "test-shovel-success"
					fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
				})

				It("sets the status condition 'Ready' to 'true'", func() {
					Expect(client.Create(ctx, &shovel)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
							&shovel,
						)

						return shovel.Status.Conditions
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
				fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(client.Create(ctx, &shovel)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
						&shovel,
					)

					return shovel.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					shovelName = "delete-shovel-http-error"
					fakeRabbitMQClient.DeleteShovelReturns(&http.Response{
						Status:     "502 Bad Gateway",
						StatusCode: http.StatusBadGateway,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("raises an event to indicate a failure to delete", func() {
					Expect(client.Delete(ctx, &shovel)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &topology.Shovel{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete shovel parameter"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					shovelName = "delete-shovel-go-error"
					fakeRabbitMQClient.DeleteShovelReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &shovel)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &topology.Shovel{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete shovel parameter"))
				})
			})

			Context("success", func() {
				BeforeEach(func() {
					shovelName = "delete-shovel-success"
					fakeRabbitMQClient.DeleteShovelReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
				})

				It("publishes a normal event", func() {
					Expect(client.Delete(ctx, &shovel)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &topology.Shovel{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					Expect(observedEvents()).To(SatisfyAll(
						Not(ContainElement("Warning FailedDelete failed to delete shovel parameter")),
						ContainElement("Normal SuccessfulDelete successfully deleted shovel parameter"),
					))
				})
			})
		})

		Context("finalizer", func() {
			BeforeEach(func() {
				shovelName = "finalizer-test"
			})

			It("sets the correct deletion finalizer to the object", func() {
				Expect(client.Create(ctx, &shovel)).To(Succeed())
				Eventually(func() []string {
					var fetched topology.Shovel
					err := client.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &fetched)
					if err != nil {
						return []string{}
					}
					return fetched.ObjectMeta.Finalizers
				}, 5).Should(ConsistOf("deletion.finalizers.shovels.rabbitmq.com"))
			})
		})
	})

	When("a shovel references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			shovelName = "test-shovel-prohibited"
			shovel = topology.Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shovelName,
					Namespace: "prohibited",
				},
				Spec: topology.ShovelSpec{
					Name:      "my-shovel-configuration",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "shovel-uri-secret"},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &shovel)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
					&shovel,
				)

				return shovel.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(topology.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a shovel references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			shovelName = "test-shovel-allowed"
			shovel = topology.Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shovelName,
					Namespace: "allowed",
				},
				Spec: topology.ShovelSpec{
					Name:      "my-shovel-configuration",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "shovel-uri-secret"},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &shovel)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
					&shovel,
				)

				return shovel.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a shovel references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			shovelName = "test-shovel-allowed-when-allow-all"
			shovel = topology.Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shovelName,
					Namespace: "prohibited",
				},
				Spec: topology.ShovelSpec{
					Name:      "my-shovel-configuration",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "shovel-uri-secret"},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &shovel)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
					&shovel,
				)

				return shovel.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
