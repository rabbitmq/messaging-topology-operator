package controller_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	k "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

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

var _ = Describe("UserController", func() {
	var (
		user          topology.User
		userName      string
		userMgr       ctrl.Manager
		managerCtx    context.Context
		managerCancel context.CancelFunc
		k8sClient     runtimeClient.Client
		userLimits    topology.UserLimits
		connections   int32
		channels      int32
	)

	initialiseManager := func(keyValPair ...string) {
		var sel labels.Selector
		if len(keyValPair) == 2 {
			var err error
			sel, err = labels.Parse(fmt.Sprintf("%s == %s", keyValPair[0], keyValPair[1]))
			Expect(err).NotTo(HaveOccurred())
		}

		var err error
		userMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{userNamespace: {
					LabelSelector: sel,
				}},
				ByObject: map[runtimeClient.Object]cache.ByObject{
					&v1beta1.RabbitmqCluster{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
					&corev1.Secret{}: {
						Namespaces: map[string]cache.Config{cache.AllNamespaces: {}},
						Label:      labels.SelectorFromSet(labels.Set{topology.TopologyOperatorLabel: topology.TopologyOperatorLabelValue}),
					},
					&corev1.Service{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
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
			Expect(userMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = userMgr.GetClient()

		Expect((&controller.TopologyReconciler{
			Client:                userMgr.GetClient(),
			APIReader:             userMgr.GetAPIReader(),
			Type:                  &topology.User{},
			Scheme:                userMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controller.UserReconciler{Client: userMgr.GetClient(), Scheme: userMgr.GetScheme()},
		}).SetupWithManager(userMgr)).To(Succeed())
	}

	initialiseUser := func() {
		user = topology.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userName,
				Namespace: userNamespace,
			},
			Spec: topology.UserSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
				UserLimits: &userLimits,
			},
		}
	}

	objectStatus := func() []topology.Condition {
		_ = k8sClient.Get(
			ctx,
			types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
			&user,
		)
		return user.Status.Conditions
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

	When("creating a user", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				userName = "test-user-http-error"
				fakeRabbitMQClient.PutUserReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
				initialiseUser()
				user.Labels = map[string]string{"test": userName}
				initialiseManager("test", userName)
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("some HTTP error"),
					})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				userName = "test-user-go-error"
				fakeRabbitMQClient.PutUserReturns(nil, errors.New("hit a exception"))
				initialiseUser()
				user.Labels = map[string]string{"test": userName}
				initialiseManager("test", userName)
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &user)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("hit a exception"),
					})))
			})
		})

		Context("user limits", func() {
			AfterEach(func() {
				userLimits.Connections = nil
				userLimits.Channels = nil
			})

			When("the user has limits defined", func() {
				BeforeEach(func() {
					userName = "test-user-limits"
					connections = 5
					channels = 10
					userLimits = topology.UserLimits{
						Connections: &connections,
						Channels:    &channels,
					}
					fakeRabbitMQClient.PutUserReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.PutUserLimitsReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.GetUserLimitsReturns(nil, rabbithole.ErrorResponse{
						StatusCode: 404,
						Message:    "Object Not Found",
						Reason:     "Not Found",
					})
					initialiseUser()
					user.Labels = map[string]string{"test": userName}
					initialiseManager("test", userName)
				})

				It("should create the user limits", func() {
					Expect(k8sClient.Create(ctx, &user)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
							&user,
						)

						return user.Status.Conditions
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					By("calling PutUserLimits with the correct user limits")
					Expect(fakeRabbitMQClient.PutUserLimitsCallCount()).To(BeNumerically(">", 0))
					_, userLimitsValues := fakeRabbitMQClient.PutUserLimitsArgsForCall(0)
					Expect(userLimitsValues).To(HaveKeyWithValue("max-connections", int(connections)))
					Expect(userLimitsValues).To(HaveKeyWithValue("max-channels", (int(channels))))
				})
			})

			When("the user already has existing limits that differ from the new limits", func() {
				BeforeEach(func() {
					userName = "test-changed-user-limits"
					connections = 5
					userLimits = topology.UserLimits{
						Connections: &connections,
						Channels:    nil,
					}
					userLimitsInfo := make([]rabbithole.UserLimitsInfo, 1)
					userLimitsInfo[0] = rabbithole.UserLimitsInfo{
						User:  userName,
						Value: rabbithole.UserLimitsValues{"max-channels": 10, "max-connections": 3},
					}
					fakeRabbitMQClient.PutUserReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.PutUserLimitsReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
					fakeRabbitMQClient.GetUserLimitsReturns(userLimitsInfo, nil)
					fakeRabbitMQClient.DeleteUserLimitsReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
					initialiseUser()
					user.Labels = map[string]string{"test": userName}
					initialiseManager("test", userName)
				})

				It("should update the existing user limit and delete the unused old limit", func() {
					Expect(k8sClient.Create(ctx, &user)).To(Succeed())
					Eventually(func() []topology.Condition {
						_ = k8sClient.Get(
							ctx,
							types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
							&user,
						)

						return user.Status.Conditions
					}).
						Within(statusEventsUpdateTimeout).
						WithPolling(time.Second).
						Should(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(topology.ConditionType("Ready")),
							"Reason": Equal("SuccessfulCreateOrUpdate"),
							"Status": Equal(corev1.ConditionTrue),
						})))
					By("calling DeleteUserLimits with the unused old user limits")
					Expect(fakeRabbitMQClient.DeleteUserLimitsCallCount()).To(BeNumerically(">", 0))
					_, userLimits := fakeRabbitMQClient.DeleteUserLimitsArgsForCall(0)
					Expect(userLimits).To(HaveLen(1))
					Expect(userLimits).To(ContainElement("max-channels"))
					By("calling PutUserLimits with the correct new user limits")
					Expect(fakeRabbitMQClient.PutUserLimitsCallCount()).To(BeNumerically(">", 0))
					_, userLimitsValues := fakeRabbitMQClient.PutUserLimitsArgsForCall(0)
					Expect(userLimitsValues).To(HaveKeyWithValue("max-connections", int(connections)))
				})
			})
		})
	})

	When("deleting a user", func() {
		JustBeforeEach(func() {
			// Must use a JustBeforeEach to extract this common behaviour
			// JustBeforeEach runs AFTER all BeforeEach have completed
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.PutUserLimitsReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.DeleteUserLimitsReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			Expect(k8sClient.Create(ctx, &user)).To(Succeed())
			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)

				return user.Status.Conditions
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
				userName = "delete-user-http-error"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
				initialiseUser()
				user.Labels = map[string]string{"test": userName}
				initialiseManager("test", userName)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete user"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				userName = "delete-user-go-error"
				fakeRabbitMQClient.DeleteUserReturns(nil, errors.New("some error"))
				initialiseUser()
				user.Labels = map[string]string{"test": userName}
				initialiseManager("test", userName)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete user"))
			})
		})

		When("the RabbitMQ Client successfully deletes a user without secret", func() {
			BeforeEach(func() {
				userName = "delete-user-success-without-secret-user-credentials"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
				initialiseUser()
				user.Labels = map[string]string{"test": userName}
				initialiseManager("test", userName)
			})

			It("raises an event to indicate a successful deletion", func() {
				Expect(k8sClient.Delete(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      user.Name + "-user-credentials",
						Namespace: user.Namespace,
					},
				})).To(Succeed())
				Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeTrue())

				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete user")),
					ContainElement("Normal SuccessfulDelete successfully deleted user"),
				))
			})
		})
	})

	When("deleting a user that was never successfully created", func() {
		BeforeEach(func() {
			userName = "delete-user-never-created"
			initialiseUser()
			user.Spec.ImportCredentialsSecret = &corev1.LocalObjectReference{
				Name: "does-not-exist",
			}
			user.Labels = map[string]string{"test": userName}
			initialiseManager("test", userName)
		})

		It("successfully deletes the user", func() {
			Expect(k8sClient.Create(ctx, &user)).To(Succeed())

			// Wait for the user to be created and have a finalizer
			Eventually(func() []string {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)
				return user.Finalizers
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(ContainElement("deletion.finalizers.users.rabbitmq.com"))

			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
				return apierrors.IsNotFound(err)
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(BeTrue())
		})
	})

	When("the user has an importCredentialsSecret — Status.Credentials.Name is set correctly", func() {
		const importSecretName = "import-creds-status-secret"

		BeforeEach(func() {
			userName = "test-import-creds-status"
			fakeRabbitMQClient.PutUserReturns(&http.Response{Status: "201 Created", StatusCode: http.StatusCreated}, nil)
			initialiseUser()
			user.Labels = map[string]string{"test": userName}
			user.Spec.ImportCredentialsSecret = &corev1.LocalObjectReference{Name: importSecretName}
			initialiseManager("test", userName)
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      importSecretName,
					Namespace: userNamespace,
					Labels: map[string]string{topology.TopologyOperatorLabel: topology.TopologyOperatorLabelValue},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"username": []byte("imported-user"), "password": []byte("imported-password")},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: importSecretName, Namespace: userNamespace},
			})).To(Succeed())
		})

		It("sets Status.Credentials.Name to the import secret (not the generated secret)", func() {
			Eventually(func() *corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &user)
				return user.Status.Credentials
			}).Within(statusEventsUpdateTimeout).WithPolling(time.Second).Should(
				HaveValue(MatchFields(IgnoreExtras, Fields{
					"Name": Equal(importSecretName),
				})),
			)
		})
	})

	When("the user has an importCredentialsSecret — PutUser is called with the imported username", func() {
		const importSecretName = "import-creds-putuser-secret"

		BeforeEach(func() {
			userName = "test-import-creds-putuser"
			fakeRabbitMQClient.PutUserReturns(&http.Response{Status: "201 Created", StatusCode: http.StatusCreated}, nil)
			initialiseUser()
			user.Labels = map[string]string{"test": userName}
			user.Spec.ImportCredentialsSecret = &corev1.LocalObjectReference{Name: importSecretName}
			initialiseManager("test", userName)
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      importSecretName,
					Namespace: userNamespace,
					Labels: map[string]string{topology.TopologyOperatorLabel: topology.TopologyOperatorLabelValue},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"username": []byte("imported-user"), "password": []byte("imported-password")},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: importSecretName, Namespace: userNamespace},
			})).To(Succeed())
		})

		It("calls PutUser with the imported username", func() {
			Eventually(objectStatus).Within(statusEventsUpdateTimeout).WithPolling(time.Second).Should(
				ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})),
			)
			Expect(fakeRabbitMQClient.PutUserCallCount()).To(BeNumerically(">", 0))
			username, _ := fakeRabbitMQClient.PutUserArgsForCall(0)
			Expect(username).To(Equal("imported-user"))
		})
	})

	When("the import secret is updated — watch triggers re-reconcile", func() {
		const importSecretName = "import-watch-secret"

		BeforeEach(func() {
			userName = "test-import-watch"
			fakeRabbitMQClient.PutUserReturns(&http.Response{Status: "201 Created", StatusCode: http.StatusCreated}, nil)
			initialiseUser()
			user.Labels = map[string]string{"test": userName}
			user.Spec.ImportCredentialsSecret = &corev1.LocalObjectReference{Name: importSecretName}
			initialiseManager("test", userName)
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      importSecretName,
					Namespace: userNamespace,
					Labels: map[string]string{topology.TopologyOperatorLabel: topology.TopologyOperatorLabelValue},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"username": []byte("imported-user"), "password": []byte("initial-password")},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &user)).To(Succeed())
			// Wait for first successful reconcile.
			Eventually(objectStatus).Within(statusEventsUpdateTimeout).WithPolling(time.Second).Should(
				ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Status": Equal(corev1.ConditionTrue),
				})),
			)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: importSecretName, Namespace: userNamespace},
			})).To(Succeed())
		})

		It("re-reconciles the user when the import secret's password is updated", func() {
			callsBefore := fakeRabbitMQClient.PutUserCallCount()

			importSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: importSecretName, Namespace: userNamespace}, importSecret)).To(Succeed())
			importSecret.Data["password"] = []byte("rotated-password")
			Expect(k8sClient.Update(ctx, importSecret)).To(Succeed())

			Eventually(func() int {
				return fakeRabbitMQClient.PutUserCallCount()
			}).Within(statusEventsUpdateTimeout).WithPolling(time.Second).
				Should(BeNumerically(">", callsBefore))

			latestIdx := fakeRabbitMQClient.PutUserCallCount() - 1
			username, _ := fakeRabbitMQClient.PutUserArgsForCall(latestIdx)
			Expect(username).To(Equal("imported-user"))
		})
	})

	It("labels the generated credentials secret with the topology operator label", func() {
		userName = "test-user-labeled-secret"
		initialiseUser()
		user.Labels = map[string]string{"test": userName}
		fakeRabbitMQClient.PutUserReturns(&http.Response{Status: "201 Created", StatusCode: http.StatusCreated}, nil)
		initialiseManager("test", userName)

		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		Eventually(objectStatus).Within(statusEventsUpdateTimeout).WithPolling(time.Second).Should(
			ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})),
		)
		Expect(k8sClient.Delete(ctx, &user)).To(Succeed())

		generatedSecret := &corev1.Secret{}
		Eventually(func() map[string]string {
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: user.Name + "-user-credentials", Namespace: user.Namespace}, generatedSecret)
			return generatedSecret.Labels
		}).Within(10 * time.Second).WithPolling(time.Second).Should(
			HaveKeyWithValue(topology.TopologyOperatorLabel, topology.TopologyOperatorLabelValue),
		)
	})

	It("sets an owner reference and does not block owner deletion", func() {
		userName = "test-owner-reference"
		initialiseUser()
		user.Labels = map[string]string{"test": userName}
		fakeRabbitMQClient.PutUserReturns(&http.Response{Status: "201 Created", StatusCode: http.StatusCreated}, nil)
		initialiseManager("test", userName)

		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		Eventually(objectStatus).
			Within(statusEventsUpdateTimeout).
			WithPolling(time.Second).
			Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})), "User should have been created and have a True Ready condition")

		generatedSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: user.Name + "-user-credentials", Namespace: user.Namespace}}
		Eventually(k.Get(generatedSecret)).
			Within(10 * time.Second).
			Should(Succeed())

		idFn := func(e any) string {
			ownRef := e.(metav1.OwnerReference)
			return ownRef.Kind
		}
		Expect(generatedSecret.OwnerReferences).To(MatchElements(idFn, IgnoreExtras,
			Elements{
				"User": MatchFields(IgnoreExtras, Fields{
					"BlockOwnerDeletion": HaveValue(BeFalseBecause("it should not block owner deletion")),
				}),
			},
		))
	})
})
