package controllers_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"time"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("vhost-controller", func() {
	var (
		vhost         topology.Vhost
		vhostName     string
		vhostMgr      ctrl.Manager
		managerCtx    context.Context
		managerCancel context.CancelFunc
		k8sClient     runtimeClient.Client
		vhostLimits   *topology.VhostLimits
	)

	initialiseManager := func(keyValPair ...string) {
		var sel labels.Selector
		if len(keyValPair) == 2 {
			var err error
			sel, err = labels.Parse(fmt.Sprintf("%s == %s", keyValPair[0], keyValPair[1]))
			Expect(err).NotTo(HaveOccurred())
		}

		var err error
		vhostMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{vhostNamespace: {
					LabelSelector: sel,
				}},
				ByObject: map[runtimeClient.Object]cache.ByObject{
					&v1beta1.RabbitmqCluster{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
					&corev1.Secret{}:           {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
					&corev1.Service{}:          {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
				},
			},
			Logger: GinkgoLogr,
			Controller: config.Controller{
				SkipNameValidation: &skipNameValidation,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		managerCtx, managerCancel = context.WithCancel(context.Background())
		go func(ctx context.Context) {
			defer GinkgoRecover()
			Expect(vhostMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = vhostMgr.GetClient()

		Expect((&controllers.TopologyReconciler{
			Client:                vhostMgr.GetClient(),
			Type:                  &topology.Vhost{},
			Scheme:                vhostMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controllers.VhostReconciler{Client: vhostMgr.GetClient()},
		}).SetupWithManager(vhostMgr)).To(Succeed())
	}

	initialiseVhost := func() {
		vhost = topology.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vhostName,
				Namespace: vhostNamespace,
			},
			Spec: topology.VhostSpec{
				Name: vhostName,
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
				VhostLimits: vhostLimits,
			},
		}
	}

	AfterEach(func() {
		managerCancel()
		// Sad workaround to avoid controllers racing for the reconciliation of other's
		// test cases. Without this wait, the last run test consistently fails because
		// the previous cancelled manager is just in time to reconcile the Queue of the
		// new/last test, and use the wrong/unexpected arguments in the queue declare call
		//
		// Eventual consistency is nice when you have good means of awaiting. That's not the
		// case with testenv and kubernetes controllers.
		<-time.After(time.Second)
	})

	Context("creation", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &vhost)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				vhostName = "test-http-error"
				fakeRabbitMQClient.PutVhostReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
				initialiseVhost()
				vhost.Labels = map[string]string{"test": "creation-http-error"}
				initialiseManager("test", "creation-http-error")
			})

			It("sets the status condition", func() {
				Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
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
				initialiseVhost()
				vhost.Labels = map[string]string{"test": "creation-go-error"}
				initialiseManager("test", "creation-go-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a go failure"),
					})))
			})
		})

		Context("vhost limits", func() {
			var connections, queues int32

			When("vhost limits are provided", func() {
				BeforeEach(func() {
					connections = 708
					queues = 509
					vhostName = "vhost-with-limits"
					vhostLimits = &topology.VhostLimits{
						Connections: &connections,
						Queues:      &queues,
					}
					fakeRabbitMQClient.PutVhostReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.PutVhostLimitsReturns(&http.Response{
						Status:     "200 OK",
						StatusCode: http.StatusOK,
					}, nil)
					fakeRabbitMQClient.GetVhostLimitsReturns(nil, rabbithole.ErrorResponse{
						StatusCode: 404,
						Message:    "Object Not Found",
						Reason:     "Not Found",
					})
					initialiseVhost()
					vhost.Labels = map[string]string{"test": "vhost-with-limits"}
					initialiseManager("test", "vhost-with-limits")
				})

				It("puts the vhost limits", func() {
					Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
							&vhost,
						)

						return vhost.Status.Conditions
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))

					Expect(fakeRabbitMQClient.PutVhostLimitsCallCount()).To(BeNumerically(">", 0))
					_, vhostLimitsValues := fakeRabbitMQClient.PutVhostLimitsArgsForCall(0)
					Expect(len(vhostLimitsValues)).To(Equal(2))
					Expect(vhostLimitsValues).To(HaveKeyWithValue("max-connections", int(connections)))
					Expect(vhostLimitsValues).To(HaveKeyWithValue("max-queues", int(queues)))
				})
			})

			When("vhost limits are not provided", func() {
				BeforeEach(func() {
					vhostName = "vhost-without-limits"
					fakeRabbitMQClient.GetVhostLimitsReturns(nil, rabbithole.ErrorResponse{
						StatusCode: 404,
						Message:    "Object Not Found",
						Reason:     "Not Found",
					})
					initialiseVhost()
					vhost.Labels = map[string]string{"test": "vhost-without-limits"}
					initialiseManager("test", "vhost-without-limits")
				})

				It("does not set vhost limits", func() {
					Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
					Expect(fakeRabbitMQClient.PutVhostLimitsCallCount()).To(Equal(0))
				})
			})

			When("vhost limits are updated", func() {
				BeforeEach(func() {
					vhostName = "vhost-updated-limits"
					queues = 613
					vhostLimits = &topology.VhostLimits{
						Connections: nil,
						Queues:      &queues,
					}

					var vhostLimitsInfo []rabbithole.VhostLimitsInfo
					vhostLimitsInfo = append(vhostLimitsInfo, rabbithole.VhostLimitsInfo{
						Vhost: vhostName,
						Value: rabbithole.VhostLimitsValues{"max-queues": 10, "max-connections": 300},
					})

					fakeRabbitMQClient.PutVhostReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.PutVhostLimitsReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.GetVhostLimitsReturns(vhostLimitsInfo, nil)
					fakeRabbitMQClient.DeleteVhostLimitsReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
					initialiseVhost()
					vhost.Labels = map[string]string{"test": "vhost-updated-limits"}
					initialiseManager("test", "vhost-updated-limits")
				})

				It("updates the provided limits and removes unspecified limits", func() {
					Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
							&vhost,
						)

						return vhost.Status.Conditions
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))

					By("deleting the outdated limits")
					Expect(fakeRabbitMQClient.DeleteVhostLimitsCallCount()).To(BeNumerically(">", 0))
					vhostname, limits := fakeRabbitMQClient.DeleteVhostLimitsArgsForCall(0)
					Expect(vhostname).To(Equal(vhostName))
					Expect(len(limits)).To(Equal(1))
					Expect(limits).To(ContainElement("max-connections"))

					By("updating the new limits")
					Expect(fakeRabbitMQClient.PutVhostLimitsCallCount()).To(BeNumerically(">", 0))
					_, vhostLimitsValues := fakeRabbitMQClient.PutVhostLimitsArgsForCall(0)
					Expect(len(vhostLimitsValues)).To(Equal(1))
					Expect(vhostLimitsValues).To(HaveKeyWithValue("max-queues", int(queues)))
				})
			})
		})
	})

	Context("deletion", func() {
		createVhost := func() {
			fakeRabbitMQClient.PutVhostReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
					&vhost,
				)

				return vhost.Status.Conditions
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
		}

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				vhostName = "delete-vhost-http-error"
				initialiseVhost()
				vhost.Labels = map[string]string{"test": vhostName}
				initialiseManager("test", vhostName)

				createVhost()

				fakeRabbitMQClient.DeleteVhostReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &vhost)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &topology.Vhost{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete vhost"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				vhostName = "delete-go-error"
				initialiseVhost()
				vhost.Labels = map[string]string{"test": vhostName}
				initialiseManager("test", vhostName)

				createVhost()

				fakeRabbitMQClient.DeleteVhostReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &vhost)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &topology.Vhost{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete vhost"))
			})
		})
	})

	When("the Vhost has DeletionPolicy set to retain", func() {
		BeforeEach(func() {
			vhostName = "vhost-with-retain-policy"
			fakeRabbitMQClient.DeleteVhostReturns(&http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
			}, nil)
			fakeRabbitMQClient.PutVhostReturns(&http.Response{StatusCode: http.StatusCreated, Status: "201 Created"}, nil)
			initialiseVhost()
			vhost.Labels = map[string]string{"test": "vhost-with-retain-policy"}
			initialiseManager("test", "vhost-with-retain-policy")
		})

		It("deletes the k8s resource but preserves the vhost in RabbitMQ server", func() {
			vhost.Spec.DeletionPolicy = "retain"
			Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
			Eventually(fakeRabbitMQClient.PutVhostCallCount).
				WithPolling(time.Second).
				Within(time.Second*3).
				Should(BeNumerically(">=", 1), "Expected to call RMQ API to create vhost")

			Expect(k8sClient.Delete(ctx, &vhost)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &vhost)
				return apierrors.IsNotFound(err)
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(BeTrue(), "vhost should not be found")

			Expect(fakeRabbitMQClient.DeleteVhostCallCount()).To(Equal(0), "Expected vhost to be deleted and no calls to RMQ API")
		})
	})
})
