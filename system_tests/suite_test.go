/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package system_tests

import (
	"context"
	"embed"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

var (
	k8sClient    client.Client
	rabbitClient *rabbithole.Client
	clientSet    *kubernetes.Clientset
	rmq          *rabbitmqv1beta1.RabbitmqCluster
	//go:embed fixtures
	fixtures embed.FS
)

var _ = BeforeSuite(func() {
	namespace := MustHaveEnv("NAMESPACE")
	scheme := runtime.NewScheme()
	Expect(topology.AddToScheme(scheme)).To(Succeed())
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
	restConfig, err := createRestConfig()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() []byte {
		output, err := kubectl(
			"-n",
			namespace,
			"get",
			"deployment",
			"-l",
			"app.kubernetes.io/name=rabbitmq-cluster-operator",
		)

		Expect(err).NotTo(HaveOccurred())

		return output
	}, 10, 1).Should(ContainSubstring("1/1"), "cluster operator not deployed")

	Eventually(func() []byte {
		output, err := kubectl(
			"-n",
			namespace,
			"get",
			"deployment",
			"-l",
			"app.kubernetes.io/name=messaging-topology-operator",
		)

		Expect(err).NotTo(HaveOccurred())

		return output
	}, 10, 1).Should(ContainSubstring("1/1"), "messaging-topology-operator not deployed")

	// setup a RabbitmqCluster used for system tests
	rmq = basicTestRabbitmqCluster("system-test", namespace)
	setupTestRabbitmqCluster(k8sClient, rmq)

	rabbitClient, err = generateRabbitClient(context.Background(), clientSet, namespace, rmq.Name)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("deleting RabbitmqCluster created in system tests")
	Expect(k8sClient.Delete(context.Background(), &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: rmq.Name, Namespace: rmq.Namespace}})).ToNot(HaveOccurred())
})
