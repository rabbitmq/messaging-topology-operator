package internal

import (
	"errors"
	"fmt"
	"os"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	vault "github.com/hashicorp/vault/api"
)

const defaultAuthPath string = "auth/kubernetes"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . SecretReader
type SecretReader interface {
	ReadSecret(path string) (*vault.Secret, error)
}

type VaultSecretReader struct {
	client *vault.Client
}

func (s VaultSecretReader) ReadSecret(path string) (*vault.Secret, error) {
	secret, err := s.client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read Vault secret: %w", err)
	}
	return secret, nil
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . SecretStoreClient
type SecretStoreClient interface {
	ReadCredentials(path string) (CredentialsProvider, error)
}

type VaultClient struct {
	Reader SecretReader
}

func (vc VaultClient) ReadCredentials(path string) (CredentialsProvider, error) {
	secret, err := vc.Reader.ReadSecret(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read Vault secret: %w", err)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data type assertion failed for Vault secret: %T %#v", secret.Data["data"], secret.Data["data"])
	}

	username, err := getValue("username", data)
	if err != nil {
		return nil, fmt.Errorf("unable to get username from Vault secret: %w", err)
	}

	password, err := getValue("password", data)
	if err != nil {
		return nil, fmt.Errorf("unable to get password from Vault secret: %w", err)
	}

	return ClusterCredentials{username: username, password: password}, nil
}

func getValue(key string, data map[string]interface{}) (string, error) {
	result, ok := data[key].(string)
	if !ok {
		return "", fmt.Errorf("expected %s to be a string but is a %T", key, data[key])
	}

	return result, nil
}

func InitializeSecretStoreClient(vaultSpec *rabbitmqv1beta1.VaultSpec) (SecretStoreClient, error) {
	role := vaultSpec.Role
	if role == "" {
		return nil, errors.New("no role value set in Vault secret backend")
	}

	// For now, the VAULT_ADDR environment variable will be the address that your pod uses to communicate with Vault.
	config := vault.DefaultConfig() // modify for more granular configuration

	vaultClient, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	var annotations map[string]string = vaultSpec.Annotations
	if annotations["vault.hashicorp.com/namespace"] != "" {
		vaultClient.SetNamespace(annotations["vault.hashicorp.com/namespace"])
	}

	// Read the service-account token from the path where the token's Kubernetes Secret is mounted.
	// By default, Kubernetes will mount this to /var/run/secrets/kubernetes.io/serviceaccount/token
	// but an administrator may have configured it to be mounted elsewhere.
	jwt, err := readServiceAccountToken("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("unable to read file containing service account token: %w", err)
	}

	loginAuthPath := defaultAuthPath
	if annotations["vault.hashicorp.com/auth-path"] != "" {
		loginAuthPath = annotations["vault.hashicorp.com/auth-path"]
	}

	vaultToken, err := readVaultClientToken(vaultClient, string(jwt), role, loginAuthPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read Vault client token: %w", err)
	}

	// use the Vault token for making all future calls to Vault
	vaultClient.SetToken(vaultToken)

	return VaultClient{Reader: &VaultSecretReader{client: vaultClient}}, nil
}

func readServiceAccountToken(path string) ([]byte, error) {
	token, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %s: %w", path, err)
	}
	return token, nil
}

func readVaultClientToken(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (string, error) {
	params := map[string]interface{}{
		"jwt":  jwtToken,
		"role": vaultRole, // the name of the role in Vault that was created with this app's Kubernetes service account bound to it
	}

	// log in to Vault's Kubernetes auth method
	resp, err := vaultClient.Logical().Write(authPath+"/login", params)
	if err != nil {
		return "", fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}

	// return the Vault client token provided in the login response
	if resp == nil || resp.Auth == nil || resp.Auth.ClientToken == "" {
		return "", fmt.Errorf("no client token found in Vault login response")
	}
	return resp.Auth.ClientToken, nil
}
