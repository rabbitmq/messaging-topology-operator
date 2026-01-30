package controller_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"time"

	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

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
	var (
		permission     topology.Permission
		user           topology.User
		permissionName string
		userName       string
		permissionMgr  ctrl.Manager
		managerCtx     context.Context
		managerCancel  context.CancelFunc
		k8sClient      runtimeClient.Client
	)

	initialiseManager := func(keyValPair ...string) {
		var sel labels.Selector
		if len(keyValPair) == 2 {
			var err error
			sel, err = labels.Parse(fmt.Sprintf("%s == %s", keyValPair[0], keyValPair[1]))
			Expect(err).NotTo(HaveOccurred())
		}

		var err error
		permissionMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{permissionNamespace: {
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
			Expect(permissionMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = permissionMgr.GetClient()

		Expect((&controller.TopologyReconciler{
			Client:                permissionMgr.GetClient(),
			Type:                  &topology.Permission{},
			Scheme:                permissionMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controller.PermissionReconciler{Client: permissionMgr.GetClient(), Scheme: permissionMgr.GetScheme()},
		}).SetupWithManager(permissionMgr)).To(Succeed())
	}

	initialisePermission := func() {
		permission = topology.Permission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      permissionName,
				Namespace: permissionNamespace,
			},
			Spec: topology.PermissionSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
				User:  "example",
				Vhost: "example",
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
		// case with testenv and kubernetes controller.
		<-time.After(time.Second)
	})

	When("validating RabbitMQ Client failures with username", func() {
		Context("creation", func() {
			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					permissionName = "test-with-username-http-error"
					fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("a failure"))
					initialisePermission()
					permission.Labels = map[string]string{"test": "test-with-username-http-error"}
					initialiseManager("test", "test-with-username-http-error")
				})

				It("sets the status condition", func() {
					Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
					permissionName = "test-with-username-go-error"
					fakeRabbitMQClient.UpdatePermissionsInReturns(nil, errors.New("a go failure"))
					initialisePermission()
					permission.Labels = map[string]string{"test": "test-with-username-go-error"}
					initialiseManager("test", "test-with-username-go-error")
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
				// Must use a JustBeforeEach to extract this common behaviour
				// JustBeforeEach runs AFTER all BeforeEach have completed
				fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
						&permission,
					)

					return permission.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
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
						Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
					initialisePermission()
					permission.Labels = map[string]string{"test": "delete-with-username-permission-http-error"}
					initialiseManager("test", "delete-with-username-permission-http-error")
				})

				It("publishes a 'warning' event", func() {
					Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
					Consistently(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete permission"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-go-error"
					fakeRabbitMQClient.ClearPermissionsInReturns(nil, errors.New("some error"))
					initialisePermission()
					permission.Labels = map[string]string{"test": "delete-with-username-go-error"}
					initialiseManager("test", "delete-with-username-go-error")
				})

				It("publishes a 'warning' event", func() {
					Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
					Consistently(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete permission"))
				})
			})
		})
	})

	When("validating RabbitMQ Client failures with userRef", func() {
		JustBeforeEach(func() {
			// Must use a JustBeforeEach to extract this common behaviour
			// JustBeforeEach runs AFTER all BeforeEach have completed
			user = topology.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: permissionNamespace,
					Labels:    map[string]string{"test": permissionName},
				},
				Spec: topology.UserSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: permissionNamespace,
					},
				},
			}
			permission = topology.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: permissionNamespace,
					Labels:    map[string]string{"test": permissionName},
				},
				Spec: topology.PermissionSpec{
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: permissionNamespace,
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
			fakeRabbitMQClient.PutUserLimitsReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.DeleteUserLimitsReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
		})

		Context("creation", func() {
			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
			})

			When("user not exist", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-create-not-exist"
					userName = "example-create-not-exist"
					initialisePermission()
					permission.Labels = map[string]string{"test": "test-with-userref-create-not-exist"}
					initialiseManager("test", "test-with-userref-create-not-exist")
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
					permissionName = "test-with-userref-create-success"
					userName = "example-create-success"
					initialisePermission()
					permission.Labels = map[string]string{"test": "test-with-userref-create-success"}
					initialiseManager("test", "test-with-userref-create-success")
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(k8sClient.Create(ctx, &user)).To(Succeed())
					user.Status.Username = userName
					Expect(k8sClient.Status().Update(ctx, &user)).To(Succeed())
					Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				user.Status.Username = userName
				Expect(k8sClient.Status().Update(ctx, &user)).To(Succeed())
				Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
						&permission,
					)

					return permission.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("Secret for User credential does not exist", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-secret"
					userName = "example-delete-secret-first"
					initialisePermission()
					permission.Labels = map[string]string{"test": permissionName}
					initialiseManager("test", permissionName)
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
					Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})

			When("User is removed first", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-user"
					userName = "example-delete-user-first"
					initialisePermission()
					permission.Labels = map[string]string{"test": permissionName}
					initialiseManager("test", permissionName)
				})

				It("publishes a 'warning' event", func() {
					Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-success"
					userName = "example-delete-success"
					initialisePermission()
					permission.Labels = map[string]string{"test": "test-with-userref-delete-success"}
					initialiseManager("test", "test-with-userref-delete-success")
				})

				It("publishes a 'warning' event", func() {
					Expect(k8sClient.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &topology.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})
		})

		When("the user already exists", func() {
			BeforeEach(func() {
				permissionName = "ownerref-with-userref-test"
				userName = "example-ownerref"
				initialisePermission()
				permission.Labels = map[string]string{"test": permissionName}
				initialiseManager("test", permissionName)
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
			})

			It("sets the correct deletion ownerref to the object", func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				user.Status.Username = userName
				Expect(k8sClient.Status().Update(ctx, &user)).To(Succeed())
				Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
				Eventually(func() []metav1.OwnerReference {
					var fetched topology.Permission
					err := k8sClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &fetched)
					if err != nil {
						return []metav1.OwnerReference{}
					}
					return fetched.ObjectMeta.OwnerReferences
				}, 5).Should(Not(BeEmpty()))
			})
		})
	})
})
