package internal_test

import (
	"context"
	"net/http"
	"net/url"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("RabbitholeClientFactory", func() {
	var (
		objs                     []runtime.Object
		fakeClient               client.Client
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		existingRabbitMQCluster  *rabbitmqv1beta1.RabbitmqCluster
		existingCredentialSecret *corev1.Secret
		existingService          *corev1.Service
		ctx                      = context.Background()
		fakeRabbitMQServer       *ghttp.Server
		fakeRabbitMQURL          *url.URL
		fakeRabbitMQPort         int
	)
	JustBeforeEach(func() {
		fakeRabbitMQServer.RouteToHandler("PUT", "/api/users/example-user", func(w http.ResponseWriter, req *http.Request) {
			user, password, ok := req.BasicAuth()
			if !(ok && user == existingRabbitMQUsername && password == existingRabbitMQPassword) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		})

		s := scheme.Scheme
		s.AddKnownTypes(rabbitmqv1beta1.SchemeBuilder.GroupVersion, &rabbitmqv1beta1.RabbitmqCluster{})
		fakeClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	})
	AfterEach(func() {
		fakeRabbitMQServer.Close()
	})

	When("the RabbitmqCluster is configured without TLS", func() {
		BeforeEach(func() {
			fakeRabbitMQServer = mockRabbitMQServer(false)

			var err error
			fakeRabbitMQURL, fakeRabbitMQPort, err = mockRabbitMQURLPort(fakeRabbitMQServer)
			Expect(err).NotTo(HaveOccurred())

			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: "rabbitmq-system",
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{
					Binding: &corev1.LocalObjectReference{
						Name: "rmq-default-user-credentials",
					},
					DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
						ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
							Name:      "rmq-service",
							Namespace: "rabbitmq-system",
						},
					},
				},
			}
			existingCredentialSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-default-user-credentials",
					Namespace: "rabbitmq-system",
				},
				Data: map[string][]byte{
					"username": []byte(existingRabbitMQUsername),
					"password": []byte(existingRabbitMQPassword),
				},
			}
			existingService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-service",
					Namespace: "rabbitmq-system",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: fakeRabbitMQURL.Hostname(),
					Ports: []corev1.ServicePort{
						{
							Name: "management",
							Port: int32(fakeRabbitMQPort),
						},
					},
				},
			}
			objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
		})

		It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
			generatedClient, err := internal.RabbitholeClientFactory(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(generatedClient).NotTo(BeNil())

			_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
		})

		When("RabbitmqCluster does not have status.binding set", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-incomplete",
						Namespace: "rabbitmq-system",
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
							ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
								Name:      "rmq-service",
								Namespace: "rabbitmq-system",
							},
						},
					},
				}
			})

			It("errors", func() {
				generatedClient, err := internal.RabbitholeClientFactory(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
				Expect(generatedClient).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("no status.binding set"))
			})
		})

		When("RabbitmqCluster does not have status.defaultUser set", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-incomplete",
						Namespace: "rabbitmq-system",
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						Binding: &corev1.LocalObjectReference{
							Name: "rmq-default-user-credentials",
						},
					},
				}
			})

			It("errors", func() {
				generatedClient, err := internal.RabbitholeClientFactory(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
				Expect(generatedClient).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("no status.defaultUser set"))
			})
		})
	})

	When("the RabbitmqCluster is configured with TLS", func() {
		BeforeEach(func() {
			fakeRabbitMQServer = mockRabbitMQServer(true)

			var err error
			fakeRabbitMQURL, fakeRabbitMQPort, err = mockRabbitMQURLPort(fakeRabbitMQServer)
			Expect(err).NotTo(HaveOccurred())

			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: "rabbitmq-system",
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{
					Binding: &corev1.LocalObjectReference{
						Name: "rmq-default-user-credentials",
					},
					DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
						ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
							Name:      "rmq-service",
							Namespace: "rabbitmq-system",
						},
					},
				},
			}
			existingCredentialSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-default-user-credentials",
					Namespace: "rabbitmq-system",
				},
				Data: map[string][]byte{
					"username": []byte(existingRabbitMQUsername),
					"password": []byte(existingRabbitMQPassword),
				},
			}
			existingService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-service",
					Namespace: "rabbitmq-system",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: fakeRabbitMQURL.Hostname(),
					Ports: []corev1.ServicePort{
						{
							Name: "management",
							Port: int32(fakeRabbitMQPort),
						},
					},
				},
			}
			existingCertSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-certs",
					Namespace: "rabbitmq-system",
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("somecertstring"),
					corev1.TLSPrivateKeyKey: []byte("somekeystring"),
				},
			}
			existingService.Spec.Ports = append(existingService.Spec.Ports, corev1.ServicePort{
				Name: "management-tls",
				Port: int32(fakeRabbitMQPort),
			})
			existingRabbitMQCluster.Spec.TLS = rabbitmqv1beta1.TLSSpec{
				SecretName:             existingCertSecret.Name,
				DisableNonTLSListeners: true,
			}
			objs = []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingCertSecret, existingService}
		})

		It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
			generatedClient, err := internal.RabbitholeClientFactory(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(generatedClient).NotTo(BeNil())

			_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
		})
	})
})
