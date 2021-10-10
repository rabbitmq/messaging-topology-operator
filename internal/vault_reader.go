package internal

import (
	"errors"
	"fmt"
	"os"

	vault "github.com/hashicorp/vault/api"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . CredentialsLocator
type CredentialsLocator interface {
	ReadCredentials(path string) (CredentialsProvider, error)
}

type VaultClient struct {
	vaultClient *vault.Client
}

func (vc VaultClient) ReadCredentials(path string) (CredentialsProvider, error) {
	secret, err := vc.vaultClient.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read Vault secret: %w", err)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data type assertion failed: %T %#v", secret.Data["data"], secret.Data["data"])
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

func InitializeCredentialsLocator() (CredentialsLocator, error) {
	// If set, the VAULT_ADDR environment variable will be the address that your pod uses to communicate with Vault.
	config := vault.DefaultConfig() // modify for more granular configuration

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	// Read the service-account token from the path where the token's Kubernetes Secret is mounted.
	// By default, Kubernetes will mount this to /var/run/secrets/kubernetes.io/serviceaccount/token
	// but an administrator may have configured it to be mounted elsewhere.
	jwt, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("unable to read file containing service account token: %w", err)
	}

	// GCH TODO Is there a sensible default value for the Vault role?
	var vaultRole string
	if len(os.Getenv("VAULT_ROLE")) > 0 {
		vaultRole = os.Getenv("VAULT_ROLE")
	} else {
		return nil, errors.New("unable to log into Vault as VAULT_ROLE is not set in the environment")
	}

	params := map[string]interface{}{
		"jwt":  string(jwt),
		"role": vaultRole, // the name of the role in Vault that was created with this app's Kubernetes service account bound to it
	}

	// log in to Vault's Kubernetes auth method
	resp, err := client.Logical().Write("auth/kubernetes/login", params)
	if err != nil {
		return nil, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if resp == nil || resp.Auth == nil || resp.Auth.ClientToken == "" {
		return nil, fmt.Errorf("login response did not return client token")
	}

	// use the Vault token for making all future calls to Vault
	client.SetToken(resp.Auth.ClientToken)

	return VaultClient{vaultClient: client}, nil
}
