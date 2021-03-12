package system_tests

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
)

func createRestConfig() (*rest.Config, error) {
	var config *rest.Config
	var err error
	var kubeconfig string

	if len(os.Getenv("KUBECONFIG")) > 0 {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube/config")
	}
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func createClientSet() (*kubernetes.Clientset, error) {
	config, err := createRestConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[error] %s \n", err)
	}

	return clientset, err
}

func kubectl(args ...string) ([]byte, error) {
	cmd := exec.Command("kubectl", args...)
	return cmd.CombinedOutput()
}

func MustHaveEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("Environment variable '%s' not found", name))
	}
	return value
}

func generateRabbitClient(ctx context.Context, clientSet *kubernetes.Clientset, rmq *v1alpha1.RabbitmqClusterReference) (*rabbithole.Client, error) {
	nodeIp := kubernetesNodeIp(ctx, clientSet)
	if nodeIp == "" {
		return nil, errors.New("failed to get kubernetes Node IP")
	}

	nodePort := managementNodePort(ctx, clientSet, rmq)
	if nodePort == "" {
		return nil, errors.New("failed to get NodePort for management")
	}

	endpoint := fmt.Sprintf("http://%s:%s", nodeIp, nodePort)

	username, password, err := getUsernameAndPassword(ctx, clientSet, rmq)
	if err != nil {
		return nil, fmt.Errorf("failed to get username and password: %v", err)
	}

	rabbitClient, err := rabbithole.NewClient(endpoint, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate rabbit client: %v", err)
	}

	return rabbitClient, nil
}

func getUsernameAndPassword(ctx context.Context, clientSet *kubernetes.Clientset, rmq *v1alpha1.RabbitmqClusterReference) (string, string, error) {
	secretName := fmt.Sprintf("%s-default-user", rmq.Name)
	secret, err := clientSet.CoreV1().Secrets(rmq.Namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	username, ok := secret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'username' in %s", secretName)
	}
	password, ok := secret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'password' in %s", secretName)
	}
	return string(username), string(password), nil
}

func managementNodePort(ctx context.Context, clientSet *kubernetes.Clientset, rmq *v1alpha1.RabbitmqClusterReference) string {
	svc, err := clientSet.CoreV1().Services(rmq.Namespace).
		Get(ctx, rmq.Name, metav1.GetOptions{})

	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	for _, port := range svc.Spec.Ports {
		if port.Name == "management" {
			return strconv.Itoa(int(port.NodePort))
		}
	}
	return ""
}

func kubernetesNodeIp(ctx context.Context, clientSet *kubernetes.Clientset) string {
	nodes, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, nodes).ToNot(BeNil())
	ExpectWithOffset(1, len(nodes.Items)).To(BeNumerically(">", 0))
	var nodeIp string
	for _, address := range nodes.Items[0].Status.Addresses {
		switch address.Type {
		case corev1.NodeExternalIP:
			return address.Address
		case corev1.NodeInternalIP:
			nodeIp = address.Address
		}
	}
	return nodeIp
}

func setupTestRabbitmqCluster(name, namespace string) *rabbitmqv1beta1.RabbitmqCluster {
	// setup a RabbitmqCluster used for system tests
	rabbitmqCluster := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas: pointer.Int32Ptr(1),
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}

	Expect(k8sClient.Create(context.Background(), rabbitmqCluster)).To(Succeed())
	Eventually(func() string {
		output, err := kubectl(
			"-n",
			rabbitmqCluster.Namespace,
			"get",
			"rabbitmqclusters",
			rabbitmqCluster.Name,
			"-ojsonpath='{.status.conditions[?(@.type==\"AllReplicasReady\")].status}'",
		)
		if err != nil {
			Expect(string(output)).To(ContainSubstring("not found"))
		}
		return string(output)
	}, 120, 10).Should(Equal("'True'"))
	return rabbitmqCluster
}
