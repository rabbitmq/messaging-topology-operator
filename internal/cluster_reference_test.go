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
		fakeCredentialsProvider  *internalfakes.FakeCredentialsProvider
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
					Namespace: "rabbitmq-system",
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{
					Binding: &corev1.LocalObjectReference{
						Name: "rmq-default-user-credentials",
					},
					DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
						ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
							Name:      "rmq",
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
					Name:      "rmq",
					Namespace: "rabbitmq-system",
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
			rmq, svc, credsProvider, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(rmq.ObjectMeta).To(Equal(existingRabbitMQCluster.ObjectMeta))
			Expect(rmq.Status).To(Equal(existingRabbitMQCluster.Status))
			Expect(svc.ObjectMeta).To(Equal(existingService.ObjectMeta))
			Expect(svc.Spec).To(Equal(existingService.Spec))

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
				_, _, _, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
				Expect(err).To(MatchError("no status.defaultUser set"))
			})
		})

		When("vault secret backend is declared on cluster spec", func() {
			var (
				err                   error
				fakeSecretStoreClient *internalfakes.FakeSecretStoreClient
				credsProv             internal.CredentialsProvider
			)

			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
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
								Name:      "rmq",
								Namespace: "rabbitmq-system",
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

				fakeCredentialsProvider = &internalfakes.FakeCredentialsProvider{}
				fakeCredentialsProvider.DataReturnsOnCall(0, []byte(existingRabbitMQUsername), true)
				fakeCredentialsProvider.DataReturnsOnCall(1, []byte(existingRabbitMQPassword), true)

				fakeSecretStoreClient.ReadCredentialsReturns(fakeCredentialsProvider, nil)
				internal.SecretStoreClientProvider = func() (internal.SecretStoreClient, error) {
					return fakeSecretStoreClient, nil
				}
			})

			AfterEach(func() {
				internal.SecretStoreClientProvider = internal.GetSecretStoreClient
			})

			JustBeforeEach(func() {
				_, _, credsProv, err = internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
			})

			It("should not return an error", func() {
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
})
