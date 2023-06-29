package rabbitmqclient_test

import (
	"context"
	"fmt"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient/rabbitmqclientfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ParseReference", func() {
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
		uriAnnotationKey         = "rabbitmq.com/operator-connection-uri"
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
			creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, "", false)
			Expect(err).NotTo(HaveOccurred())

			Expect(tlsEnabled).To(BeFalse())
			usernameBytes, _ := creds["username"]
			passwordBytes, _ := creds["password"]
			uriBytes, _ := creds["uri"]
			Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
			Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
			Expect(uriBytes).To(Equal("http://rmq.rabbitmq-system.svc:15672"))
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
				_, _, err := rabbitmqclient.ParseReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, "", false)
				Expect(err).To(MatchError(rabbitmqclient.NoServiceReferenceSetError))
			})
		})

		When("vault secret backend is declared on cluster spec", func() {
			var (
				err                   error
				fakeSecretStoreClient *rabbitmqclientfakes.FakeSecretStoreClient
				tlsEnabled            bool
				creds                 map[string]string
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

				fakeSecretStoreClient = &rabbitmqclientfakes.FakeSecretStoreClient{}
				fakeSecretStoreClient.ReadCredentialsReturns(existingRabbitMQUsername, existingRabbitMQPassword, nil)
				rabbitmqclient.SecretStoreClientProvider = func() (rabbitmqclient.SecretStoreClient, error) {
					return fakeSecretStoreClient, nil
				}
			})

			AfterEach(func() {
				rabbitmqclient.SecretStoreClientProvider = rabbitmqclient.GetSecretStoreClient
			})

			JustBeforeEach(func() {
				creds, tlsEnabled, err = rabbitmqclient.ParseReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, "", false)
			})

			It("should not return an error", func() {
				Expect(tlsEnabled).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the expected credentials", func() {
				usernameBytes, _ := creds["username"]
				passwordBytes, _ := creds["password"]
				uriBytes, _ := creds["uri"]
				Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
				Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
				Expect(uriBytes).To(Equal("http://rmq.rabbitmq-system.svc:15672"))
			})

			When("RabbitmqCluster does not have status.defaultUser set", func() {
				BeforeEach(func() {
					*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rmq-vault-incomplete-status",
							Namespace: namespace,
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
					fakeSecretStoreClient = &rabbitmqclientfakes.FakeSecretStoreClient{}
					fakeSecretStoreClient.ReadCredentialsReturns(existingRabbitMQUsername, existingRabbitMQPassword, nil)
					rabbitmqclient.SecretStoreClientProvider = func() (rabbitmqclient.SecretStoreClient, error) {
						return fakeSecretStoreClient, nil
					}
				})

				It("errors", func() {
					_, _, err := rabbitmqclient.ParseReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, "", false)
					Expect(err).To(MatchError(rabbitmqclient.NoServiceReferenceSetError))
				})
			})
		})
	})

	When("the RabbitmqCluster is configured with only TLS", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName:             "a-tls-secret",
						DisableNonTLSListeners: true,
					},
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
							Name: "management-tls",
							Port: int32(15671),
						},
					},
				},
			}
			objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
		})

		DescribeTable("returns correct creds in connectionCredentials",
			func(connectUsingHTTP, expectedTLSEnabled bool, expectedUri string) {
				creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
					existingRabbitMQCluster.Namespace,
					"",
					connectUsingHTTP)
				Expect(err).NotTo(HaveOccurred())
				usernameBytes, _ := creds["username"]
				passwordBytes, _ := creds["password"]
				uriBytes, _ := creds["uri"]
				Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
				Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
				Expect(uriBytes).To(Equal(expectedUri))

				Expect(tlsEnabled).To(Equal(expectedTLSEnabled))
			},
			Entry("When connectingUsingHTTP is true", true, true, "https://rmq.rabbitmq-system.svc:15671"),
			Entry("When connectingUsingHTTP is false", false, true, "https://rmq.rabbitmq-system.svc:15671"),
		)
	})

	When("the RabbitmqCluster is configured with TLS and other listeners are enabled", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName:             "a-tls-secret",
						DisableNonTLSListeners: false,
					},
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
							Name: "management-tls",
							Port: int32(15671),
						},
						{
							Name: "management",
							Port: int32(15672),
						},
					},
				},
			}
			objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
		})

		DescribeTable("returns correct creds in connectionCredentials",
			func(connectUsingHTTP, expectedTLSEnabled bool, expectedUri string) {
				creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
					existingRabbitMQCluster.Namespace,
					"",
					connectUsingHTTP)
				Expect(err).NotTo(HaveOccurred())
				usernameBytes, _ := creds["username"]
				passwordBytes, _ := creds["password"]
				uriBytes, _ := creds["uri"]
				Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
				Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
				Expect(uriBytes).To(Equal(expectedUri))

				Expect(tlsEnabled).To(Equal(expectedTLSEnabled))
			},
			Entry("When connectingUsingHTTP is true", true, false, "http://rmq.rabbitmq-system.svc:15672"),
			Entry("When connectingUsingHTTP is false", false, true, "https://rmq.rabbitmq-system.svc:15671"),
		)
	})

	When("the RabbitmqCluster is configured with management path_prefix", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Rabbitmq: rabbitmqv1beta1.RabbitmqClusterConfigurationSpec{
						AdditionalConfig: `
							management.path_prefix = /my/prefix
						`,
					},
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

		It("returns correct creds in connectionCredentials", func() {
			creds, _, err := rabbitmqclient.ParseReference(ctx, fakeClient,
				topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
				existingRabbitMQCluster.Namespace,
				"",
				false)
			Expect(err).NotTo(HaveOccurred())

			usernameBytes, _ := creds["username"]
			passwordBytes, _ := creds["password"]
			uriBytes, _ := creds["uri"]
			Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
			Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
			Expect(uriBytes).To(Equal("http://rmq.rabbitmq-system.svc:15672/my/prefix"))
		})
	})

	Context("spec.rabbitmqClusterReference.connectionSecret is set", func() {
		When("uri has no scheme defined", func() {
			BeforeEach(func() {
				noSchemeSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-connection-info",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"uri":      []byte("10.0.0.0:15672"),
						"username": []byte("test-user"),
						"password": []byte("test-password"),
					},
				}
				objs = []runtime.Object{noSchemeSecret}
			})

			It("returns the expected connection information", func() {
				creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "rmq-connection-info",
						},
					},
					namespace,
					"",
					false)
				Expect(err).NotTo(HaveOccurred())

				Expect(tlsEnabled).To(BeFalse())
				returnedUser, _ := creds["username"]
				returnedPass, _ := creds["password"]
				returnedURI, _ := creds["uri"]
				Expect(returnedUser).To(Equal("test-user"))
				Expect(returnedPass).To(Equal("test-password"))
				Expect(returnedURI).To(Equal("http://10.0.0.0:15672"))
			})
		})

		When("uri sets http as the scheme", func() {
			BeforeEach(func() {
				httpSchemeSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-connection-info",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"uri":      []byte("http://10.0.0.0:15672"),
						"username": []byte("test-user"),
						"password": []byte("test-password"),
					},
				}
				objs = []runtime.Object{httpSchemeSecret}
			})

			It("returns the expected connection information", func() {
				creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "rmq-connection-info",
						},
					},
					namespace,
					"",
					false)
				Expect(err).NotTo(HaveOccurred())

				Expect(tlsEnabled).To(BeFalse())
				returnedUser, _ := creds["username"]
				returnedPass, _ := creds["password"]
				returnedURI, _ := creds["uri"]
				Expect(returnedUser).To(Equal("test-user"))
				Expect(returnedPass).To(Equal("test-password"))
				Expect(returnedURI).To(Equal("http://10.0.0.0:15672"))
			})
		})

		When("uri sets https as the scheme", func() {
			BeforeEach(func() {
				httpsSchemeSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-connection-info",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"uri":      []byte("https://10.0.0.0:15671"),
						"username": []byte("test-user"),
						"password": []byte("test-password"),
					},
				}
				objs = []runtime.Object{httpsSchemeSecret}
			})

			It("returns the expected connection information", func() {
				creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "rmq-connection-info",
						},
					},
					namespace,
					"",
					false)
				Expect(err).NotTo(HaveOccurred())

				Expect(tlsEnabled).To(BeTrue())
				returnedUser, _ := creds["username"]
				returnedPass, _ := creds["password"]
				returnedURI, _ := creds["uri"]
				Expect(returnedUser).To(Equal("test-user"))
				Expect(returnedPass).To(Equal("test-password"))
				Expect(returnedURI).To(Equal("https://10.0.0.0:15671"))
			})
		})
	})

	Context("cluster domain", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = new(rabbitmqv1beta1.RabbitmqCluster)
			existingRabbitMQCluster.Name = "bunny"
			existingRabbitMQCluster.Namespace = namespace
			existingRabbitMQCluster.Status.Binding = &corev1.LocalObjectReference{
				Name: "bunny-default-user-credentials",
			}
			existingRabbitMQCluster.Status.DefaultUser = &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
				ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
					Name:      "bunny",
					Namespace: namespace,
				}}

			existingCredentialSecret = new(corev1.Secret)
			existingCredentialSecret.Name = "bunny-default-user-credentials"
			existingCredentialSecret.Namespace = namespace
			existingCredentialSecret.Data = map[string][]byte{
				"username": []byte(existingRabbitMQUsername),
				"password": []byte(existingRabbitMQPassword),
			}

			existingService = new(corev1.Service)
			existingService.Name = "bunny"
			existingService.Namespace = namespace
			existingService.Spec.ClusterIP = "1.2.3.4"
			existingService.Spec.Ports = []corev1.ServicePort{
				{
					Name: "management",
					Port: int32(15672),
				}}

			objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
		})

		It("generates an address with cluster domain suffix", func() {
			someDomain := ".example.com"

			creds, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
				topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
				existingRabbitMQCluster.Namespace,
				someDomain,
				false)
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsEnabled).To(BeFalse(), "expected TLS to not be enabled")
			Expect(creds).ToNot(BeNil())

			uri, ok := creds["uri"]
			Expect(ok).To(BeTrue(), "expected Credentials Provider to contain a key 'uri'")
			Expect(uri).To(Equal(fmt.Sprintf("http://bunny.%s.svc.example.com:15672", namespace)))
		})

		When("the domain suffix is not present", func() {
			It("generates the shortname", func() {
				credsProvider, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
					existingRabbitMQCluster.Namespace,
					"",
					false)
				Expect(err).NotTo(HaveOccurred())
				Expect(tlsEnabled).To(BeFalse(), "expected TLS to not be enabled")
				Expect(credsProvider).ToNot(BeNil())

				uri, ok := credsProvider["uri"]
				Expect(ok).To(BeTrue(), "expected Credentials Provider to contain a key 'uri'")
				Expect(uri).To(Equal(fmt.Sprintf("http://bunny.%s.svc:15672", namespace)))
			})
		})
	})

	Context("namespace permissions", func() {
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
		})

		When("requested namespace is prohibited", func() {
			BeforeEach(func() {
				existingRabbitMQCluster.ObjectMeta.Annotations = map[string]string{}
				objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
			})
			It("should return an error about a cluster being prohibited", func() {
				_, _, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{
						Name:      existingRabbitMQCluster.Name,
						Namespace: existingRabbitMQCluster.Namespace,
					},
					"prohibited-namespace",
					"",
					false)
				Expect(err).To(MatchError(rabbitmqclient.ResourceNotAllowedError))
			})
		})

		When("there is a list of allowed namespaces", func() {
			BeforeEach(func() {
				existingRabbitMQCluster.ObjectMeta.Annotations = map[string]string{
					"rabbitmq.com/topology-allowed-namespaces": "allowed1,allowed2",
				}
				objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
			})
			When("requested namespace is allowed", func() {
				It("works", func() {
					_, _, err := rabbitmqclient.ParseReference(ctx, fakeClient,
						topology.RabbitmqClusterReference{
							Name:      existingRabbitMQCluster.Name,
							Namespace: existingRabbitMQCluster.Namespace,
						},
						"allowed1",
						"",
						false)
					Expect(err).NotTo(HaveOccurred())

					_, _, err = rabbitmqclient.ParseReference(ctx, fakeClient,
						topology.RabbitmqClusterReference{
							Name:      existingRabbitMQCluster.Name,
							Namespace: existingRabbitMQCluster.Namespace,
						},
						"allowed2",
						"",
						false)
					Expect(err).NotTo(HaveOccurred())
				})
			})
			When("requested namespace is not allowed", func() {
				It("returns error", func() {
					_, _, err := rabbitmqclient.ParseReference(ctx, fakeClient,
						topology.RabbitmqClusterReference{
							Name:      existingRabbitMQCluster.Name,
							Namespace: existingRabbitMQCluster.Namespace,
						},
						"allowed3",
						"",
						false)
					Expect(err).To(MatchError(rabbitmqclient.ResourceNotAllowedError))
				})
			})
		})

		When("all namespaces are allowed", func() {
			BeforeEach(func() {
				existingRabbitMQCluster.ObjectMeta.Annotations = map[string]string{
					"rabbitmq.com/topology-allowed-namespaces": "*",
				}
				objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
			})

			It("works with any namespace", func() {
				_, _, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{
						Name:      existingRabbitMQCluster.Name,
						Namespace: existingRabbitMQCluster.Namespace,
					},
					"any-namespace-will-be-fine",
					"",
					false)
				Expect(err).NotTo(HaveOccurred())

				_, _, err = rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{
						Name:      existingRabbitMQCluster.Name,
						Namespace: existingRabbitMQCluster.Namespace,
					},
					"another-namespace",
					"",
					false)
				Expect(err).NotTo(HaveOccurred())
			})
		})

	})

	When("the RabbitmqCluster is annotated with connection uri override", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: namespace,
					Annotations: map[string]string{
						uriAnnotationKey: "http://a-rabbitmq-test:2333",
					},
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

		It("returns correct credentials", func() {
			creds, tlsOn, err := rabbitmqclient.ParseReference(ctx, fakeClient,
				topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
				existingRabbitMQCluster.Namespace,
				"",
				false)
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsOn).To(BeFalse())

			usernameBytes, _ := creds["username"]
			passwordBytes, _ := creds["password"]
			uriBytes, _ := creds["uri"]
			Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
			Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
			Expect(uriBytes).To(Equal("http://a-rabbitmq-test:2333"))
		})

		When("annotated URI has no scheme", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq",
						Namespace: namespace,
						Annotations: map[string]string{
							uriAnnotationKey: "a-rabbitmq-test:7890",
						},
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
			})

			It("sets http as the scheme", func() {
				creds, tlsOn, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
					existingRabbitMQCluster.Namespace,
					"",
					false)
				Expect(err).NotTo(HaveOccurred())
				Expect(tlsOn).To(BeFalse())

				usernameBytes, _ := creds["username"]
				passwordBytes, _ := creds["password"]
				uriBytes, _ := creds["uri"]
				Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
				Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
				Expect(uriBytes).To(Equal("http://a-rabbitmq-test:7890"))
			})
		})

		When("annotated URI has https as scheme", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq",
						Namespace: namespace,
						Annotations: map[string]string{
							uriAnnotationKey: "https://a-rabbitmq-test:2333",
						},
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
			})

			It("returns correct credentials", func() {
				creds, tlsOn, err := rabbitmqclient.ParseReference(ctx, fakeClient,
					topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name},
					existingRabbitMQCluster.Namespace,
					"",
					false)
				Expect(err).NotTo(HaveOccurred())
				Expect(tlsOn).To(BeTrue())

				usernameBytes, _ := creds["username"]
				passwordBytes, _ := creds["password"]
				uriBytes, _ := creds["uri"]
				Expect(usernameBytes).To(Equal(existingRabbitMQUsername))
				Expect(passwordBytes).To(Equal(existingRabbitMQPassword))
				Expect(uriBytes).To(Equal("https://a-rabbitmq-test:2333"))
			})
		})

	})
})

var _ = Describe("AllowedNamespace", func() {
	When("rabbitmqcluster reference namespace is an empty string", func() {
		It("returns true", func() {
			Expect(rabbitmqclient.AllowedNamespace(topology.RabbitmqClusterReference{Name: "a-name"}, "", nil)).To(BeTrue())
		})
	})

	When("rabbitmqcluster reference namespace matches requested namespace", func() {
		It("returns true", func() {
			Expect(rabbitmqclient.AllowedNamespace(topology.RabbitmqClusterReference{Name: "a-name", Namespace: "a-ns"}, "a-ns", nil)).To(BeTrue())
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
			Expect(rabbitmqclient.AllowedNamespace(ref, "test", cluster)).To(BeTrue())
			Expect(rabbitmqclient.AllowedNamespace(ref, "test0", cluster)).To(BeTrue())
			Expect(rabbitmqclient.AllowedNamespace(ref, "test1", cluster)).To(BeTrue())
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
			Expect(rabbitmqclient.AllowedNamespace(ref, "notThere", cluster)).To(BeTrue())
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
			Expect(rabbitmqclient.AllowedNamespace(ref, "anything", cluster)).To(BeTrue())
			Expect(rabbitmqclient.AllowedNamespace(ref, "whatever", cluster)).To(BeTrue())
		})
	})
})
