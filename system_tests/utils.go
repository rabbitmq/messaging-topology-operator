package system_tests

import (
	"context"
	"errors"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func generateRabbitClient(ctx context.Context, clientSet *kubernetes.Clientset, rmq *v1beta1.RabbitmqClusterReference) (*rabbithole.Client, error) {
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

func getUsernameAndPassword(ctx context.Context, clientSet *kubernetes.Clientset, rmq *v1beta1.RabbitmqClusterReference) (string, string, error) {
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

func managementNodePort(ctx context.Context, clientSet *kubernetes.Clientset, rmq *v1beta1.RabbitmqClusterReference) string {
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
