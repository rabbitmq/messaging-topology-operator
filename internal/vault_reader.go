package internal

import (
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"os"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	vault "github.com/hashicorp/vault/api"
)

// VaultReader Public Interface that retrieves credentials from Vault
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . VaultReader
type VaultReader interface {
	ReadCredentials(path string) (CredentialsProvider, error)
}
type VaultReaderImpl struct {
	vaultDelegate VaultDelegate
}

// GetVaultReader Returns a singleton instance of VaultReader or an error it could not be initialized
func GetVaultReader(vaultSpec *rabbitmqv1beta1.VaultSpec) (VaultReader, error) {
	once.Do(func() {
		singleton, singletonError = InitializeVaultReader(vaultSpec)
	})
	return singleton, singletonError
}

// VaultDelegateFactory Public Global Variable that allow us to inject our own VaultDelegateFactoryMethod instance and
// ServiceAccountToken provider method VaultReaderImpl delegates to VaultDelegateFactoryMethod and ServiceAccountToken
var VaultDelegateFactory VaultDelegateFactoryMethod = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (VaultDelegate, error) {
	return NewVaultDelegateImpl(vaultSpec)
}
var ServiceAccountToken = readServiceAccountToken

var (
	once           sync.Once
	singleton      VaultReader
	singletonError error
)

func InitializeVaultReader(vaultSpec *rabbitmqv1beta1.VaultSpec) (VaultReader, error) {

	delegate, err := VaultDelegateFactory(vaultSpec)
	if err != nil {
		return nil, fmt.Errorf("unable to create VaultDelegateFactoryMethod. Reason: %+v", err)
	}
	FirstLoginAttemptResultCh := make(chan error, 1)

	vr := VaultReaderImpl{
		vaultDelegate: delegate,
	}
	go vr.renewToken(FirstLoginAttemptResultCh)
	err = <-FirstLoginAttemptResultCh
	if err != nil {
		return nil, fmt.Errorf("unable to login to Vault: %w", err)
	}
	return vr, nil

}

func (vr VaultReaderImpl) ReadCredentials(path string) (CredentialsProvider, error) {
	secret, err := vr.vaultDelegate.ReadSecret(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read Vault secret: %w", err)
	}

	if secret == nil {
		return nil, errors.New("returned Vault secret is nil")
	}

	if secret != nil && secret.Warnings != nil && len(secret.Warnings) > 0 {
		return nil, fmt.Errorf("warnings were returned from Vault: %v", secret.Warnings)
	}

	if secret.Data == nil {
		return nil, errors.New("returned Vault secret has a nil Data map")
	}

	if len(secret.Data) == 0 {
		return nil, errors.New("returned Vault secret has an empty Data map")
	}

	if secret.Data["data"] == nil {
		return nil, fmt.Errorf("returned Vault secret has a Data map that contains no value for key 'data'. Available keys are: %v", availableKeys(secret.Data))
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data type assertion failed for Vault secret of type: %T and value %#v read from path %s", secret.Data["data"], secret.Data["data"], path)
	}

	username, err := getValue("username", data)
	if err != nil {
		return nil, fmt.Errorf("unable to get username from Vault secret: %w", err)
	}

	password, err := getValue("password", data)
	if err != nil {
		return nil, fmt.Errorf("unable to get password from Vault secret: %w", err)
	}

	return ClusterCredentials{
		data: map[string][]byte{
			"username": []byte(username),
			"password": []byte(password),
		},
	}, nil
}

func getValue(key string, data map[string]interface{}) (string, error) {
	result, ok := data[key].(string)
	if !ok {
		return "", fmt.Errorf("expected %s to be a string but is a %T", key, data[key])
	}

	return result, nil
}

func availableKeys(m map[string]interface{}) []string {
	result := make([]string, len(m))
	i := 0
	for k := range m {
		result[i] = k
		i++
	}
	return result
}

func (vr *VaultReaderImpl) login() (*vault.Secret, error) {
	logger := ctrl.LoggerFrom(nil)

	// GCH TODO return to this...
	// role := vaultSpec.Role
	// if role == "" {
	// 	return nil, errors.New("no role value set in Vault secret backend")
	// }
	role := "messaging-topology-operator"

	jwt, err := ServiceAccountToken()
	if err != nil {
		return nil, fmt.Errorf("unable to read file containing service account token: %w", err)
	}

	logger.Info("Authenticating to Vault")
	return vr.authenticateToVault(string(jwt), role)
}

func (vr *VaultReaderImpl) renewToken(initialLoginErrorCh chan<- error) {
	logger := ctrl.LoggerFrom(nil)
	sentFirstLoginAttemptErr := false

	for {
		vaultLoginResp, err := vr.login()
		if err != nil {
			logger.Error(err, "unable to authenticate to Vault server")
		}

		if !sentFirstLoginAttemptErr {
			initialLoginErrorCh <- err
			sentFirstLoginAttemptErr = true
			if err != nil {
				// Initial login attempt failed so fail fast and don't try to manage (non-existent) token lifecycle
				logger.Info("Lifecycle management of Vault token will not be carried out")
				return
			}
			logger.Info("Initiating lifecycle management of Vault token")
		}

		err = vr.vaultDelegate.ManageTokenLifecycle(vaultLoginResp)
		if err != nil {
			logger.Error(err, "unable to start managing the Vault token lifecycle")
		}

		// Reduce load on Vault server in a problem situation where repeated login attempts may be made
		time.Sleep(2 * time.Second)
	}
}

func readServiceAccountToken() ([]byte, error) {
	// Read the service-account token from the path where the token's Kubernetes Secret is mounted.
	// By default, Kubernetes will mount this to /var/run/secrets/kubernetes.io/serviceaccount/token
	// but an administrator may have configured it to be mounted elsewhere.
	path := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	token, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %s: %w", path, err)
	}
	return token, nil
}

func  (vr *VaultReaderImpl) authenticateToVault(jwtToken string, vaultRole string) (*vault.Secret, error) {
	params := map[string]interface{}{
		"jwt":  jwtToken,
		"role": vaultRole, // the name of the role in Vault that was created with this app's Kubernetes service account bound to it
	}
	return vr.vaultDelegate.Authenticate(params)
}

// VaultDelegate Abstraction to model Vault collaborator/dependency
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . VaultDelegate
type VaultDelegate interface {
	// Authenticate and return an Auth token
	Authenticate(params map[string]interface{}) (*vault.Secret, error)
	// ManageTokenLifecycle - Keep Auth token up-to-date and renew it when expires
	ManageTokenLifecycle(*vault.Secret) error
	// ReadSecret - Lookup a secret by its path
	ReadSecret(path string) (*vault.Secret, error)
}

// VaultDelegateFactoryMethod Factory method that instantiates a VaultDelegate implementation
type VaultDelegateFactoryMethod func(vaultSpec *rabbitmqv1beta1.VaultSpec) (VaultDelegate, error)

// VaultDelegateImpl Implementation of VaultDelegate that talks to Vault server via Vault Go client
type VaultDelegateImpl struct {
	authPath string
	client   *vault.Client
}

const defaultAuthPath string = "auth/kubernetes"

func NewVaultDelegateImpl(vaultSpec *rabbitmqv1beta1.VaultSpec) (VaultDelegate, error) {
	// VAULT_ADDR environment variable will be the address that pod uses to communicate with Vault.

	config := vault.DefaultConfig() // modify for more granular configuration
	vaultClient, err := vault.NewClient(config)
	if err != nil {
		singletonError = fmt.Errorf("unable to initialize Vault delegate: %w", err)
		return nil, err
	}
	var annotations = vaultSpec.Annotations
	if annotations["vault.hashicorp.com/namespace"] != "" {
		vaultClient.SetNamespace(annotations["vault.hashicorp.com/namespace"])
	}
	loginAuthPath := defaultAuthPath
	annotations = vaultSpec.Annotations
	if annotations["vault.hashicorp.com/auth-path"] != "" {
		loginAuthPath = annotations["vault.hashicorp.com/auth-path"]
	}

	var delegate = &VaultDelegateImpl{
		authPath: loginAuthPath,
		client:   vaultClient,
	}
	return delegate, nil

}
func (c *VaultDelegateImpl) Authenticate(params map[string]interface{}) (*vault.Secret, error) {
	vaultSecret, err := c.client.Logical().Write(c.authPath+"/login", params)
	if err != nil {
		return nil, err
	}
	if vaultSecret == nil || vaultSecret.Auth == nil || vaultSecret.Auth.ClientToken == "" {
		return nil, fmt.Errorf("no client token found in Vault secret")
	}
	c.client.SetToken(vaultSecret.Auth.ClientToken)
	return vaultSecret, nil
}
func (c *VaultDelegateImpl) ReadSecret(path string) (*vault.Secret, error) {
	secret, err := c.client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read Vault secret: %w", err)
	}
	return secret, nil
}
func (c *VaultDelegateImpl) ManageTokenLifecycle(token *vault.Secret) error {
	logger := ctrl.LoggerFrom(nil)

	if token == nil || token.Auth == nil {
		logger.Info("No Vault secret available. Re-attempting login")
		return nil
	}

	renew := token.Auth.Renewable
	if !renew {
		logger.Info("Token is not configured to be renewable. Re-attempting login")
		return nil
	}

	watcher, err := c.client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: token,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize new lifetime watcher for renewing auth token: %w", err)
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		// `DoneCh` will return if renewal fails, or if the remaining lease duration is
		// under a built-in threshold and either renewing is not extending it or
		// renewing is disabled.  In any case, the caller needs to attempt to log in again.
		case err := <-watcher.DoneCh():
			if err != nil {
				logger.Error(err, "Failed to renew Vault token. Re-attempting login")
				return nil
			}
			logger.Info("Token can no longer be renewed. Re-attempting login.")
			return nil

		// Successfully completed renewal
		case renewal := <-watcher.RenewCh():
			logger.Info("Successfully renewed Vault token", "renewal info", renewal)
		}
	}
}
