package system_tests

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/testutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func generateRabbitClient(ctx context.Context, clientSet *kubernetes.Clientset, namespace, name string) (*rabbithole.Client, error) {
	endpoint, err := managementEndpoint(ctx, clientSet, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get management endpoint: %w", err)
	}

	username, password, err := getUsernameAndPassword(ctx, clientSet, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get username and password: %v", err)
	}

	rabbitClient, err := rabbithole.NewClient(endpoint, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate rabbit client: %v", err)
	}

	return rabbitClient, nil
}

func getUsernameAndPassword(ctx context.Context, clientSet *kubernetes.Clientset, namespace, name string) (string, string, error) {
	secretName := fmt.Sprintf("%s-default-user", name)
	secret, err := clientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
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

func managementEndpoint(ctx context.Context, clientSet *kubernetes.Clientset, namespace, name string) (string, error) {
	nodeIp := kubernetesNodeIp(ctx, clientSet)
	if nodeIp == "" {
		return "", errors.New("failed to get kubernetes Node IP")
	}

	nodePort := managementNodePort(ctx, clientSet, namespace, name)
	if nodePort == "" {
		return "", errors.New("failed to get NodePort for management")
	}

	return fmt.Sprintf("http://%s:%s", nodeIp, nodePort), nil
}

func managementNodePort(ctx context.Context, clientSet *kubernetes.Clientset, namespace, name string) string {
	svc, err := clientSet.CoreV1().Services(namespace).
		Get(ctx, name, metav1.GetOptions{})

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

func basicTestRabbitmqCluster(name, namespace string) *rabbitmqv1beta1.RabbitmqCluster {
	return &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas: pointer.Int32Ptr(1),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Type: corev1.ServiceTypeNodePort,
			},
			Rabbitmq: rabbitmqv1beta1.RabbitmqClusterConfigurationSpec{
				AdditionalPlugins: []rabbitmqv1beta1.Plugin{"rabbitmq_federation", "rabbitmq_shovel"},
			},
		},
	}
}

func setupTestRabbitmqCluster(k8sClient client.Client, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) {
	// setup a RabbitmqCluster used for system tests
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
			Expect(string(output)).To(ContainSubstring("NotFound"))
		}
		return string(output)
	}, 120, 10).Should(Equal("'True'"))
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rabbitmqCluster)).To(Succeed())
}

func updateTestRabbitmqCluster(k8sClient client.Client, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) {
	// update a RabbitmqCluster used for system tests
	Expect(k8sClient.Update(context.Background(), rabbitmqCluster)).To(Succeed())
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
			Expect(string(output)).To(ContainSubstring("NotFound"))
		}
		return string(output)
	}, 120, 10).Should(Equal("'True'"))
}

func createTLSSecret(secretName, secretNamespace, hostname string) (string, []byte, []byte) {
	// create cert files
	serverCertPath, serverCertFile := testutils.CreateCertFile(2, "server.crt")
	serverKeyPath, serverKeyFile := testutils.CreateCertFile(2, "server.key")
	caCertPath, caCertFile := testutils.CreateCertFile(2, "ca.crt")

	// generate and write cert and key to file
	caCert, caKey := testutils.CreateCertificateChain(2, hostname, caCertFile, serverCertFile, serverKeyFile)

	tmpfile, err := ioutil.TempFile("", "ca.key")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(caKey)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	err = tmpfile.Close()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	// create CA tls secret
	ExpectWithOffset(1, k8sCreateTLSSecret(secretName+"-ca", secretNamespace, caCertPath, tmpfile.Name())).To(Succeed())
	// create k8s tls secret
	ExpectWithOffset(1, k8sCreateTLSSecret(secretName, secretNamespace, serverCertPath, serverKeyPath)).To(Succeed())

	// remove cert files
	ExpectWithOffset(1, os.Remove(serverKeyPath)).To(Succeed())
	ExpectWithOffset(1, os.Remove(serverCertPath)).To(Succeed())
	return caCertPath, caCert, caKey
}

func k8sSecretExists(secretName, secretNamespace string) (bool, error) {
	output, err := kubectl(
		"-n",
		secretNamespace,
		"get",
		"secret",
		secretName,
	)

	if err != nil {
		ExpectWithOffset(1, string(output)).To(ContainSubstring("NotFound"))
		return false, nil
	}

	return true, nil
}

func k8sCreateTLSSecret(secretName, secretNamespace, certPath, keyPath string) error {
	// delete secret if it exists
	secretExists, err := k8sSecretExists(secretName, secretNamespace)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	if secretExists {
		ExpectWithOffset(1, k8sDeleteSecret(secretName, secretNamespace)).To(Succeed())
	}

	// create secret
	output, err := kubectl(
		"-n",
		secretNamespace,
		"create",
		"secret",
		"tls",
		secretName,
		fmt.Sprintf("--cert=%+v", certPath),
		fmt.Sprintf("--key=%+v", keyPath),
	)

	if err != nil {
		return fmt.Errorf("Failed with error: %v\nOutput: %v\n", err.Error(), string(output))
	}

	return nil
}

func k8sDeleteSecret(secretName, secretNamespace string) error {
	output, err := kubectl(
		"-n",
		secretNamespace,
		"delete",
		"secret",
		secretName,
	)

	if err != nil {
		return fmt.Errorf("Failed with error: %v\nOutput: %v\n", err.Error(), string(output))
	}

	return nil
}
