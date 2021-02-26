package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"testing"

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
)

var _ = BeforeSuite(func() {
	namespace := MustHaveEnv("NAMESPACE")
	scheme := runtime.NewScheme()
	Expect(topologyv1beta1.AddToScheme(scheme)).To(Succeed())
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
	rmq = &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-test",
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas: pointer.Int32Ptr(1),
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}

	Expect(k8sClient.Create(context.Background(), rmq)).To(Succeed())
	Eventually(func() string {
		output, err := kubectl(
			"-n",
			rmq.Namespace,
			"get",
			"rabbitmqclusters",
			rmq.Name,
			"-ojsonpath='{.status.conditions[?(@.type==\"AllReplicasReady\")].status}'",
		)
		if err != nil {
			Expect(string(output)).To(ContainSubstring("not found"))
		}
		return string(output)
	}, 60, 10).Should(Equal("'True'"))

	rabbitClient, err = generateRabbitClient(context.Background(), clientSet, &topologyv1beta1.RabbitmqClusterReference{Name: rmq.Name, Namespace: rmq.Namespace})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("deleting RabbitmqCluster created in system tests")
	Expect(k8sClient.Delete(context.Background(), &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: rmq.Name, Namespace: rmq.Namespace}})).ToNot(HaveOccurred())
})
