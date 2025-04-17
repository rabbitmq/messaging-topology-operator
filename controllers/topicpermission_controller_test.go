package controllers_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"io"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
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
	var (
		topicperm          topology.TopicPermission
		user               topology.User
		name               string
		userName           string
		topicPermissionMgr ctrl.Manager
		managerCtx         context.Context
		managerCancel      context.CancelFunc
		k8sClient          runtimeClient.Client
	)

	BeforeEach(func() {
		var err error
		topicPermissionMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{topicPermissionNamespace: {}},
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
			Expect(topicPermissionMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = topicPermissionMgr.GetClient()

		Expect((&controllers.TopologyReconciler{
			Client:                topicPermissionMgr.GetClient(),
			Type:                  &topology.TopicPermission{},
			Scheme:                topicPermissionMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controllers.TopicPermissionReconciler{Client: topicPermissionMgr.GetClient(), Scheme: topicPermissionMgr.GetScheme()},
		}).SetupWithManager(topicPermissionMgr)).To(Succeed())
	})

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

	When("validating RabbitMQ Client failures with username", func() {
		JustBeforeEach(func() {
			topicperm = topology.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: topicPermissionNamespace,
				},
				Spec: topology.TopicPermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
					User:  "example",
					Vhost: "example",
					Permissions: topology.TopicPermissionConfig{
						Exchange: "some",
					},
				},
			}
		})

		Context("creation", func() {
			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					name = "test-with-username-http-error"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("a failure"))
				})

				It("sets the status condition", func() {
					Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
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
					name = "test-with-username-go-error"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(nil, errors.New("a go failure"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
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
		})

		Context("deletion", func() {
			JustBeforeEach(func() {
				fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
						&topicperm,
					)

					return topicperm.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
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
						Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("publishes a 'warning' event", func() {
					Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
					Consistently(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete topicpermission"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					name = "delete-with-username-go-error"
					fakeRabbitMQClient.DeleteTopicPermissionsInReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
					Consistently(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(BeFalse())
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
					Namespace: topicPermissionNamespace,
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: topicPermissionNamespace,
					},
				},
			}
			topicperm = topology.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: topicPermissionNamespace,
				},
				Spec: topology.TopicPermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: topicPermissionNamespace,
					},
					UserReference: &corev1.LocalObjectReference{
						Name: userName,
					},
					Vhost: "example",
					Permissions: topology.TopicPermissionConfig{
						Exchange: "some",
					},
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
			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
			})

			When("user not exist", func() {
				BeforeEach(func() {
					name = "test-with-userref-create-not-exist"
					userName = "topic-perm-example-create-not-exist"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(ContainElement(MatchFields(IgnoreExtras, Fields{
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
					Expect(k8sClient.Create(ctx, &user)).To(Succeed())
					user.Status.Username = userName
					Expect(k8sClient.Status().Update(ctx, &user)).To(Succeed())
					Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
				})
			})
		})

		Context("deletion", func() {
			JustBeforeEach(func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				user.Status.Username = userName
				Expect(k8sClient.Status().Update(ctx, &user)).To(Succeed())
				Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
						&topicperm,
					)

					return topicperm.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
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
					// We used to delete a Secret right here, before this PR:
					// https://github.com/rabbitmq/messaging-topology-operator/pull/710
					//
					// That PR refactored tests and provided controller isolation in tests, so that
					// other controllers i.e. User controller, won't interfere with resources
					// created/deleted/modified as part of this suite. Therefore, we don't need to
					// delete the Secret objects because, after PR 710, the Secret object is never
					// created, which meets the point of this test: when the Secret does not exist
					// and Permission is deleted
					Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(BeTrue())
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
					Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
						return apierrors.IsNotFound(err)
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(BeTrue())

					Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(BeTrue())

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
					Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &topology.TopicPermission{})
						return apierrors.IsNotFound(err)
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(BeTrue())

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

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
				Expect(k8sClient.Delete(ctx, &topicperm)).To(Succeed())
			})

			It("sets the correct deletion ownerref to the object", func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				user.Status.Username = userName
				Expect(k8sClient.Status().Update(ctx, &user)).To(Succeed())

				Expect(k8sClient.Create(ctx, &topicperm)).To(Succeed())
				Eventually(func() []metav1.OwnerReference {
					var fetched topology.TopicPermission
					err := k8sClient.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &fetched)
					if err != nil {
						return []metav1.OwnerReference{}
					}
					return fetched.ObjectMeta.OwnerReferences
				}).
					Within(5 * time.Second).
					WithPolling(time.Second).
					Should(Not(BeEmpty()))
			})
		})
	})
})
