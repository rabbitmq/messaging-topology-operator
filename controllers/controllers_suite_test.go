/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/internal/internalfakes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var (
	testEnv                   *envtest.Environment
	client                    runtimeClient.Client
	clientSet                 *kubernetes.Clientset
	ctx                       = context.Background()
	fakeRabbitMQClient        internalfakes.FakeRabbitMQClient
	fakeRabbitMQClientFactory = func(ctx context.Context, c runtimeClient.Client, rmq topology.RabbitmqClusterReference, namespace string) (internal.RabbitMQClient, error) {
		return &fakeRabbitMQClient, nil
	}
	fakeRecorder *record.FakeRecorder
)

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(topology.AddToScheme(scheme.Scheme)).To(Succeed())

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

	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	client = mgr.GetClient()
	Expect(client).ToNot(BeNil())

	close(done)
}, 60)

var _ = BeforeEach(func() {
	fakeRabbitMQClient = internalfakes.FakeRabbitMQClient{}
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
