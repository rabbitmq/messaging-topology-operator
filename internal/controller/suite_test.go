/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controller_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"go/build"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient/rabbitmqclientfakes"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite", Label("controller-suite"))
}

var (
	testEnv *envtest.Environment
	ctx     context.Context
	cancel  context.CancelFunc
	//mgr                       ctrl.Manager
	fakeRabbitMQClient        *rabbitmqclientfakes.FakeClient
	fakeRabbitMQClientError   error
	fakeRabbitMQClientFactory = func(connectionCreds map[string]string, tlsEnabled bool, certPool *x509.CertPool) (rabbitmqclient.Client, error) {
		fakeRabbitMQClientFactoryArgsForCall = append(fakeRabbitMQClientFactoryArgsForCall, struct {
			arg1 map[string]string
			arg2 bool
			arg3 *x509.CertPool
		}{connectionCreds, tlsEnabled, certPool})
		return fakeRabbitMQClient, fakeRabbitMQClientError
	}
	// Shameless copy of what counterfeiter does for mocking
	fakeRabbitMQClientFactoryArgsForCall []struct {
		arg1 map[string]string
		arg2 bool
		arg3 *x509.CertPool
	}
	fakeRecorder              *record.FakeRecorder
	statusEventsUpdateTimeout      = 20 * time.Second
	skipNameValidation        bool = true
)

const (
	bindingNamespace           = "binding-test"
	exchangeNamespace          = "exchange-test"
	permissionNamespace        = "permission-test"
	policyNamespace            = "policy-test"
	queueNamespace             = "queue-test"
	userNamespace              = "user-test"
	vhostNamespace             = "vhost-test"
	schemaReplicationNamespace = "schema-replication-test"
	federationNamespace        = "federation-test"
	shovelNamespace            = "shovel-test"
	topicPermissionNamespace   = "topic-permission-test"
	superStreamNamespace       = "super-stream-test"
	topologyNamespace          = "topology-reconciler-test"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	By("bootstrapping test environment")
	operatorCrdsGlob := fmt.Sprintf(
		"%s/%s",
		build.Default.GOPATH,
		"pkg/mod/github.com/rabbitmq/cluster-operator/*/config/crd/bases",
	)
	operatorCrds, err := filepath.Glob(operatorCrdsGlob)
	Expect(err).ToNot(HaveOccurred())
	Expect(operatorCrds).ToNot(BeEmpty())

	operatorCrds = append(operatorCrds, filepath.Join("..", "..", "config", "crd", "bases"))
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     operatorCrds,
		ErrorIfCRDPathMissing: true,
		Config: &rest.Config{
			Host: fmt.Sprintf("localhost:218%d", GinkgoParallelProcess()),
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	// These lines are important to ensure that managers and controllers
	// know how to convert k8s objects from JSON to Go objects
	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(topology.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(topologyv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(rabbitmqv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

	namespaces := []string{
		bindingNamespace,
		exchangeNamespace,
		permissionNamespace,
		policyNamespace,
		queueNamespace,
		userNamespace,
		vhostNamespace,
		schemaReplicationNamespace,
		federationNamespace,
		shovelNamespace,
		topicPermissionNamespace,
		superStreamNamespace,
		topologyNamespace,
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	client, err := runtimeClient.New(cfg, runtimeClient.Options{})
	Expect(err).ToNot(HaveOccurred())

	for _, n := range namespaces {
		_, err := clientSet.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		rmq := rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example-rabbit",
				Namespace: n,
				Annotations: map[string]string{
					"rabbitmq.com/topology-allowed-namespaces": "allowed",
				},
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
				TLS: rabbitmqv1beta1.TLSSpec{
					SecretName: "i-do-not-exist-but-its-fine",
				},
			},
		}
		Expect(createRabbitmqClusterResources(client, &rmq)).To(Succeed())

		rmq = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-all-rabbit",
				Namespace: n,
				Annotations: map[string]string{
					"rabbitmq.com/topology-allowed-namespaces": "*",
				},
			},
		}
		Expect(createRabbitmqClusterResources(client, &rmq)).To(Succeed())
	}

	fakeRecorder = record.NewFakeRecorder(128)

	//Expect(superStreamReconciler.SetupWithManager(mgr)).To(Succeed())

	komega.SetClient(client)
	komega.SetContext(ctx)

	// used in schema-replication-controller test
	endpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: schemaReplicationNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username":  []byte("a-random-user"),
			"password":  []byte("a-random-password"),
			"endpoints": []byte("a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672"),
		},
	}
	Expect(client.Create(ctx, &endpointsSecret)).To(Succeed())

	// used in federation-controller test
	federationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: federationNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"uri": []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
		},
	}
	Expect(client.Create(ctx, &federationUriSecret)).To(Succeed())

	// used in shovel-controller test
	shovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: shovelNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"srcUri":  []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
			"destUri": []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
		},
	}
	Expect(client.Create(ctx, &shovelUriSecret)).To(Succeed())
})

var _ = BeforeEach(func() {
	fakeRabbitMQClient = &rabbitmqclientfakes.FakeClient{}
	fakeRabbitMQClientError = nil
	fakeRabbitMQClientFactoryArgsForCall = nil
})

var _ = AfterEach(func() {
	for len(fakeRecorder.Events) > 0 {
		// Drain any unused events
		<-fakeRecorder.Events
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})

func observedEvents() []string {
	var events []string
	for len(fakeRecorder.Events) > 0 {
		events = append(events, <-fakeRecorder.Events)
	}
	return events
}

func FakeRabbitMQClientFactoryArgsForCall(i int) (map[string]string, bool, *x509.CertPool) {
	// More shameless copy of counterfeiter code generation idea
	argsForCall := fakeRabbitMQClientFactoryArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func createRabbitmqClusterResources(client runtimeClient.Client, rabbitmqObj *rabbitmqv1beta1.RabbitmqCluster) error {
	rmqCreds := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-user-credentials", rabbitmqObj.Name),
			Namespace: rabbitmqObj.Namespace,
		},
	}
	err := client.Create(ctx, &rmqCreds)
	if err != nil {
		return err
	}

	rmqSrv := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rabbitmqObj.Name,
			Namespace: rabbitmqObj.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 15672,
					Name: "management",
				},
				{
					Port: 15671,
					Name: "management-tls",
				},
			},
		},
	}
	err = client.Create(ctx, &rmqSrv)
	if err != nil {
		return err
	}

	err = client.Create(ctx, rabbitmqObj)
	if err != nil {
		return err
	}

	rabbitmqObj.Status = rabbitmqv1beta1.RabbitmqClusterStatus{
		Binding: &corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-user-credentials", rabbitmqObj.Name),
		},
		DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
			ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
				Name:      rabbitmqObj.Name,
				Namespace: rabbitmqObj.Namespace,
			},
		},
	}
	rabbitmqObj.Status.SetConditions([]runtime.Object{})
	err = client.Status().Update(ctx, rabbitmqObj)
	if err != nil {
		return err
	}
	return nil
}
