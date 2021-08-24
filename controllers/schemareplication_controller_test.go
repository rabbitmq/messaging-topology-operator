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

var _ = Describe("schema-replication-controller", func() {
	var replication topology.SchemaReplication
	var replicationName string

	When("validating RabbitMQ Client failures", func() {
		JustBeforeEach(func() {
			replication = topology.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "default",
				},
				Spec: topology.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret", // created in 'BeforeSuite'
					},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
				},
			}
		})

		When("creation", func() {
			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					replicationName = "test-replication-http-error"
					fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("some HTTP error"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &replication)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
							&replication,
						)

						return replication.Status.Conditions
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
					replicationName = "test-replication-go-error"
					fakeRabbitMQClient.PutGlobalParameterReturns(nil, errors.New("some go failure here"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &replication)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
							&replication,
						)

						return replication.Status.Conditions
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
					replicationName = "test-replication-success"
					fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
				})

				It("sets the status condition 'Ready' to 'true'", func() {
					Expect(client.Create(ctx, &replication)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
							&replication,
						)

						return replication.Status.Conditions
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
				fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(client.Create(ctx, &replication)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
						&replication,
					)

					return replication.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					replicationName = "delete-replication-http-error"
					fakeRabbitMQClient.DeleteGlobalParameterReturns(&http.Response{
						Status:     "502 Bad Gateway",
						StatusCode: http.StatusBadGateway,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("raises an event to indicate a failure to delete", func() {
					Expect(client.Delete(ctx, &replication)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &topology.SchemaReplication{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete global parameter 'schema_definition_sync_upstream'"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					replicationName = "delete-replication-go-error"
					fakeRabbitMQClient.DeleteGlobalParameterReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &replication)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &topology.SchemaReplication{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete global parameter 'schema_definition_sync_upstream'"))
				})
			})

			Context("success", func() {
				BeforeEach(func() {
					replicationName = "delete-replication-success"
					fakeRabbitMQClient.DeleteGlobalParameterReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
				})

				It("publishes a normal event", func() {
					Expect(client.Delete(ctx, &replication)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &topology.SchemaReplication{})
						return apierrors.IsNotFound(err)
					}, 5).Should(BeTrue())
					Expect(observedEvents()).To(SatisfyAll(
						Not(ContainElement("Warning FailedDelete failed to delete global parameter 'schema_definition_sync_upstream'")),
						ContainElement("Normal SuccessfulDelete successfully delete 'schema_definition_sync_upstream' global parameter"),
					))
				})
			})
		})

		Context("finalizer", func() {
			BeforeEach(func() {
				replicationName = "finalizer-test"
			})

			It("sets the correct deletion finalizer to the object", func() {
				Expect(client.Create(ctx, &replication)).To(Succeed())
				Eventually(func() []string {
					var fetched topology.SchemaReplication
					err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &fetched)
					if err != nil {
						return []string{}
					}
					return fetched.ObjectMeta.Finalizers
				}, 5).Should(ConsistOf("deletion.finalizers.schemareplications.rabbitmq.com"))
			})
		})
	})

	When("a schema replication references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			replicationName = "test-replication-prohibited"
			replication = topology.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "prohibited",
				},
				Spec: topology.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret", // created in 'BeforeSuite'
					},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(topology.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a schema replication references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			replicationName = "test-replication-allowed"
			replication = topology.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "allowed",
				},
				Spec: topology.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret",
					},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a schema replication references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			replicationName = "test-replication-allowed-when-allow-all"
			replication = topology.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "prohibited",
				},
				Spec: topology.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret",
					},
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
