/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers_test

import (
	"context"
	"crypto/x509"
	"go/build"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/internal/internalfakes"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var (
	testEnv                   *envtest.Environment
	client                    runtimeClient.Client
	clientSet                 *kubernetes.Clientset
	ctx                       = context.Background()
	fakeRabbitMQClient        *internalfakes.FakeRabbitMQClient
	fakeRabbitMQClientError   error
	fakeRabbitMQClientFactory = func(rmq *rabbitmqv1beta1.RabbitmqCluster, svc *corev1.Service, secret *corev1.Secret, hostname string, certPool *x509.CertPool) (internal.RabbitMQClient, error) {
		return fakeRabbitMQClient, fakeRabbitMQClientError
	}
	fakeRecorder *record.FakeRecorder
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "github.com", "rabbitmq", "cluster-operator@v1.8.3", "config", "crd", "bases"),
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(topology.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(rabbitmqv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

	clientSet, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	fakeRecorder = record.NewFakeRecorder(128)

	err = (&controllers.BindingReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.ExchangeReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.PermissionReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.PolicyReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.QueueReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.UserReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.VhostReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.SchemaReplicationReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.FederationReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.ShovelReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	client = mgr.GetClient()
	Expect(client).ToNot(BeNil())

	rmqCreds := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-rabbit-user-credentials",
			Namespace: "default",
		},
	}
	Expect(client.Create(ctx, &rmqCreds)).To(Succeed())

	rmqSrv := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-rabbit",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 15671,
				},
			},
		},
	}
	Expect(client.Create(ctx, &rmqSrv)).To(Succeed())

	rmq := rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-rabbit",
			Namespace: "default",
			Annotations: map[string]string{
				"rabbitmq.com/topology-allowed-namespaces": "allowed",
			},
		},
	}
	Expect(client.Create(ctx, &rmq)).To(Succeed())

	rmq.Status = rabbitmqv1beta1.RabbitmqClusterStatus{
		Binding: &corev1.LocalObjectReference{
			Name: "example-rabbit-user-credentials",
		},
		DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
			ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
				Name:      "example-rabbit",
				Namespace: "default",
			},
		},
	}
	rmq.Status.SetConditions([]runtime.Object{})
	Expect(client.Status().Update(ctx, &rmq)).To(Succeed())

	rmqCreds = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-rabbit-user-credentials",
			Namespace: "default",
		},
	}
	Expect(client.Create(ctx, &rmqCreds)).To(Succeed())

	rmqSrv = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-rabbit",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 15671,
				},
			},
		},
	}
	Expect(client.Create(ctx, &rmqSrv)).To(Succeed())

	rmq = rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-rabbit",
			Namespace: "default",
			Annotations: map[string]string{
				"rabbitmq.com/topology-allowed-namespaces": "*",
			},
		},
	}
	Expect(client.Create(ctx, &rmq)).To(Succeed())

	rmq.Status = rabbitmqv1beta1.RabbitmqClusterStatus{
		Binding: &corev1.LocalObjectReference{
			Name: "allow-all-rabbit-user-credentials",
		},
		DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
			ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
				Name:      "allow-all-rabbit",
				Namespace: "default",
			},
		},
	}
	rmq.Status.SetConditions([]runtime.Object{})
	Expect(client.Status().Update(ctx, &rmq)).To(Succeed())

	allowedNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "allowed",
		},
	}
	Expect(client.Create(ctx, &allowedNamespace)).To(Succeed())

	prohibitedNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prohibited",
		},
	}
	Expect(client.Create(ctx, &prohibitedNamespace)).To(Succeed())

	endpointsSecretBody := map[string][]byte{
		"username":  []byte("a-random-user"),
		"password":  []byte("a-random-password"),
		"endpoints": []byte("a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672"),
	}

	// used in schema-replication-controller test
	endpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: endpointsSecretBody,
	}
	Expect(client.Create(ctx, &endpointsSecret)).To(Succeed())

	allowedEndpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: "allowed",
		},
		Type: corev1.SecretTypeOpaque,
		Data: endpointsSecretBody,
	}
	Expect(client.Create(ctx, &allowedEndpointsSecret)).To(Succeed())

	prohibitedEndpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: "prohibited",
		},
		Type: corev1.SecretTypeOpaque,
		Data: endpointsSecretBody,
	}
	Expect(client.Create(ctx, &prohibitedEndpointsSecret)).To(Succeed())

	federationUriSecretBody := map[string][]byte{
		"uri": []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
	}

	// used in federation-controller test
	federationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: federationUriSecretBody,
	}
	Expect(client.Create(ctx, &federationUriSecret)).To(Succeed())

	allowedFederationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: "allowed",
		},
		Type: corev1.SecretTypeOpaque,
		Data: federationUriSecretBody,
	}
	Expect(client.Create(ctx, &allowedFederationUriSecret)).To(Succeed())

	prohibitedFederationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: "prohibited",
		},
		Type: corev1.SecretTypeOpaque,
		Data: federationUriSecretBody,
	}
	Expect(client.Create(ctx, &prohibitedFederationUriSecret)).To(Succeed())

	shovelUriSecretBody := map[string][]byte{
		"srcUri":  []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
		"destUri": []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
	}

	// used in shovel-controller test
	shovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: shovelUriSecretBody,
	}
	Expect(client.Create(ctx, &shovelUriSecret)).To(Succeed())

	allowedShovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: "allowed",
		},
		Type: corev1.SecretTypeOpaque,
		Data: shovelUriSecretBody,
	}
	Expect(client.Create(ctx, &allowedShovelUriSecret)).To(Succeed())

	prohibitedShovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: "prohibited",
		},
		Type: corev1.SecretTypeOpaque,
		Data: shovelUriSecretBody,
	}
	Expect(client.Create(ctx, &prohibitedShovelUriSecret)).To(Succeed())
})

var _ = BeforeEach(func() {
	fakeRabbitMQClient = &internalfakes.FakeRabbitMQClient{}
	fakeRabbitMQClientError = nil
})

var _ = AfterEach(func() {
	for len(fakeRecorder.Events) > 0 {
		// Drain any unused events
		<-fakeRecorder.Events
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

func observedEvents() []string {
	var events []string
	for len(fakeRecorder.Events) > 0 {
		events = append(events, <-fakeRecorder.Events)
	}
	return events
}
