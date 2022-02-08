package internal_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/internal/internalfakes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ParseRabbitmqClusterReference", func() {
	var (
		objs                     []runtime.Object
		fakeClient               client.Client
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		existingRabbitMQCluster  *rabbitmqv1beta1.RabbitmqCluster
		existingCredentialSecret *corev1.Secret
		existingService          *corev1.Service
		ctx                      = context.Background()
		namespace                = "rabbitmq-system"
	)
	JustBeforeEach(func() {
		s := scheme.Scheme
		s.AddKnownTypes(rabbitmqv1beta1.SchemeBuilder.GroupVersion, &rabbitmqv1beta1.RabbitmqCluster{})
		fakeClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	})

	When("the RabbitmqCluster is configured without TLS", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: namespace,
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{
					Binding: &corev1.LocalObjectReference{
						Name: "rmq-default-user-credentials",
					},
					DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
						ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
							Name:      "rmq",
							Namespace: namespace,
						},
					},
				},
			}
			existingCredentialSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-default-user-credentials",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"username": []byte(existingRabbitMQUsername),
					"password": []byte(existingRabbitMQPassword),
				},
			}
			existingService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: namespace,
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Ports: []corev1.ServicePort{
						{
							Name: "management",
							Port: int32(15672),
						},
					},
				},
			}
			objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
		})

		It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
			credsProvider, tlsEnabled, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(tlsEnabled).To(BeFalse())
			usernameBytes, _ := credsProvider.Data("username")
			passwordBytes, _ := credsProvider.Data("password")
			Expect(usernameBytes).To(Equal([]byte(existingRabbitMQUsername)))
			Expect(passwordBytes).To(Equal([]byte(existingRabbitMQPassword)))
		})

		When("RabbitmqCluster does not have status.defaultUser set", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-incomplete",
						Namespace: namespace,
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						Binding: &corev1.LocalObjectReference{
							Name: "rmq-default-user-credentials",
						},
					},
				}
			})

			It("errors", func() {
				_, _, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
				Expect(err).To(MatchError("no status.defaultUser set"))
			})
		})

		When("vault secret backend is declared on cluster spec", func() {
			var (
				err                   error
				fakeSecretStoreClient *internalfakes.FakeSecretStoreClient
				credsProv             internal.ConnectionCredentials
				tlsEnabled            bool
			)

			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq",
						Namespace: namespace,
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						Binding: &corev1.LocalObjectReference{
							Name: "rmq-default-user-credentials",
						},
						DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
							ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
								Name:      "rmq",
								Namespace: namespace,
							},
						},
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						SecretBackend: rabbitmqv1beta1.SecretBackend{
							Vault: &rabbitmqv1beta1.VaultSpec{
								Role:            "sausage",
								DefaultUserPath: "/some/path",
							},
						},
					},
				}

				fakeSecretStoreClient = &internalfakes.FakeSecretStoreClient{}
				fakeSecretStoreClient.ReadCredentialsReturns(existingRabbitMQUsername, existingRabbitMQPassword, nil)
				internal.SecretStoreClientProvider = func() (internal.SecretStoreClient, error) {
					return fakeSecretStoreClient, nil
				}
			})

			AfterEach(func() {
				internal.SecretStoreClientProvider = internal.GetSecretStoreClient
			})

			JustBeforeEach(func() {
				credsProv, tlsEnabled, err = internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
			})

			It("should not return an error", func() {
				Expect(tlsEnabled).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the expected credentials", func() {
				usernameBytes, _ := credsProv.Data("username")
				passwordBytes, _ := credsProv.Data("password")
				Expect(usernameBytes).To(Equal([]byte(existingRabbitMQUsername)))
				Expect(passwordBytes).To(Equal([]byte(existingRabbitMQPassword)))
			})
		})
	})
	When("spec.rabbitmqClusterReference.connectionSecret is set instead of cluster name", func() {
		BeforeEach(func() {
			connectionSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-connection-info",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"uri":      []byte("10.0.0.0:15671"),
					"username": []byte("test-user"),
					"password": []byte("test-password"),
				},
			}
			objs = []runtime.Object{connectionSecret}
		})

		It("returns the expected connection information", func() {
			credsProvider, tlsEnabled, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient,
				topology.RabbitmqClusterReference{
					ConnectionSecret: &corev1.LocalObjectReference{
						Name: "rmq-connection-info",
					},
				},
				namespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(tlsEnabled).To(BeFalse())
			returnedUser, _ := credsProvider.Data("username")
			returnedPass, _ := credsProvider.Data("password")
			returnedURI, _ := credsProvider.Data("uri")
			Expect(string(returnedUser)).To(Equal("test-user"))
			Expect(string(returnedPass)).To(Equal("test-password"))
			Expect(string(returnedURI)).To(Equal("10.0.0.0:15671"))
		})
	})
})

var _ = Describe("AllowedNamespace", func() {
	When("rabbitmqcluster reference namespace is an empty string", func() {
		It("returns true", func() {
			Expect(internal.AllowedNamespace(topology.RabbitmqClusterReference{Name: "a-name"}, "", nil)).To(BeTrue())
		})
	})

	When("rabbitmqcluster reference namespace matches requested namespace", func() {
		It("returns true", func() {
			Expect(internal.AllowedNamespace(topology.RabbitmqClusterReference{Name: "a-name", Namespace: "a-ns"}, "a-ns", nil)).To(BeTrue())
		})
	})

	When("requested namespace matches topology-allowed-namespaces annotation", func() {
		It("returns true", func() {
			cluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rabbitmq.com/topology-allowed-namespaces": "test,test0,test1",
					},
				},
			}
			ref := topology.RabbitmqClusterReference{Name: "a-name"}
			Expect(internal.AllowedNamespace(ref, "test", cluster)).To(BeTrue())
			Expect(internal.AllowedNamespace(ref, "test0", cluster)).To(BeTrue())
			Expect(internal.AllowedNamespace(ref, "test1", cluster)).To(BeTrue())
		})
	})

	When("request namespace is not listed in topology-allowed-namespaces annotations", func() {
		It("returns false", func() {
			cluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rabbitmq.com/topology-allowed-namespaces": "test,test0,test1",
					},
				},
			}
			ref := topology.RabbitmqClusterReference{Name: "a-name"}
			Expect(internal.AllowedNamespace(ref, "notThere", cluster)).To(BeTrue())
		})
	})

	When("topology-allowed-namespaces is set to *", func() {
		It("returns true", func() {
			cluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rabbitmq.com/topology-allowed-namespaces": "*",
					},
				},
			}
			ref := topology.RabbitmqClusterReference{Name: "a-name"}
			Expect(internal.AllowedNamespace(ref, "anything", cluster)).To(BeTrue())
			Expect(internal.AllowedNamespace(ref, "whatever", cluster)).To(BeTrue())
		})
	})

})
