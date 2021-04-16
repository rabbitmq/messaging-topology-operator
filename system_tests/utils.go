package system_tests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
			Expect(string(output)).To(ContainSubstring("not found"))
		}
		return string(output)
	}, 120, 10).Should(Equal("'True'"))
}

func createTLSSecret(secretName, secretNamespace, hostname string) (string, []byte, []byte) {
	// create cert files
	serverCertPath, serverCertFile := createCertFile(2, "server.crt")
	serverKeyPath, serverKeyFile := createCertFile(2, "server.key")
	caCertPath, caCertFile := createCertFile(2, "ca.crt")

	// generate and write cert and key to file
	caCert, caKey, err := createCertificateChain(hostname, caCertFile, serverCertFile, serverKeyFile)
	ExpectWithOffset(1, err).To(Succeed())
	// create k8s tls secret
	ExpectWithOffset(1, k8sCreateTLSSecret(secretName, secretNamespace, serverCertPath, serverKeyPath)).To(Succeed())

	// remove cert files
	ExpectWithOffset(1, os.Remove(serverKeyPath)).To(Succeed())
	ExpectWithOffset(1, os.Remove(serverCertPath)).To(Succeed())
	return caCertPath, caCert, caKey
}

func updateTLSSecret(secretName, secretNamespace, hostname string, caCert, caKey []byte) {
	serverCertPath, serverCertFile := createCertFile(2, "server.crt")
	serverKeyPath, serverKeyFile := createCertFile(2, "server.key")

	ExpectWithOffset(1, generateCertandKey(hostname, caCert, caKey, serverCertFile, serverKeyFile)).To(Succeed())
	ExpectWithOffset(1, k8sCreateTLSSecret(secretName, secretNamespace, serverCertPath, serverKeyPath)).To(Succeed())

	ExpectWithOffset(1, os.Remove(serverKeyPath)).To(Succeed())
	ExpectWithOffset(1, os.Remove(serverCertPath)).To(Succeed())
}

func createCertFile(offset int, fileName string) (string, *os.File) {
	tmpDir, err := ioutil.TempDir("", "certs")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred())
	path := filepath.Join(tmpDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0755)
	ExpectWithOffset(offset, err).ToNot(HaveOccurred())
	return path, file
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
		ExpectWithOffset(1, string(output)).To(ContainSubstring("not found"))
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

// generate a pair of certificate and key, given a cacert
func generateCertandKey(hostname string, caCert, caKey []byte, certWriter, keyWriter io.Writer) error {
	caPriv, err := helpers.ParsePrivateKeyPEM(caKey)
	if err != nil {
		return err
	}

	caPub, err := helpers.ParseCertificatePEM(caCert)
	if err != nil {
		return err
	}

	s, err := local.NewSigner(caPriv, caPub, signer.DefaultSigAlgo(caPriv), nil)
	if err != nil {
		return err
	}

	// create server cert
	serverReq := &csr.CertificateRequest{
		Names: []csr.Name{
			{
				C:  "UK",
				ST: "London",
				L:  "London",
				O:  "VMWare",
				OU: "RabbitMQ",
			},
		},
		CN:         "tests-server",
		Hosts:      []string{hostname},
		KeyRequest: &csr.KeyRequest{A: "rsa", S: 2048},
	}

	serverCsr, serverKey, err := csr.ParseRequest(serverReq)
	if err != nil {
		return err
	}

	signReq := signer.SignRequest{Hosts: serverReq.Hosts, Request: string(serverCsr)}
	serverCert, err := s.Sign(signReq)
	if err != nil {
		return err
	}

	_, err = certWriter.Write(serverCert)
	if err != nil {
		return err
	}
	_, err = keyWriter.Write(serverKey)
	if err != nil {
		return err
	}
	return nil
}

// creates a CA cert, and uses it to sign another cert
// it returns the generated ca cert and key so they can be reused
func createCertificateChain(hostname string, caCertWriter, certWriter, keyWriter io.Writer) ([]byte, []byte, error) {
	// create a CA cert
	caReq := &csr.CertificateRequest{
		Names: []csr.Name{
			{
				C:  "UK",
				ST: "London",
				L:  "London",
				O:  "VMWare",
				OU: "RabbitMQ",
			},
		},
		CN:         "tests-CA",
		Hosts:      []string{hostname},
		KeyRequest: &csr.KeyRequest{A: "rsa", S: 2048},
	}

	caCert, _, caKey, err := initca.New(caReq)
	if err != nil {
		return nil, nil, err
	}

	_, err = caCertWriter.Write(caCert)
	if err != nil {
		return nil, nil, err
	}

	if err := generateCertandKey(hostname, caCert, caKey, certWriter, keyWriter); err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}
