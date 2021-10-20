package internal_test

import (
	"errors"

	vault "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/internal/internalfakes"
)

var _ = Describe("VaultReader", func() {
	var (
		err                      error
		credsProvider            internal.CredentialsProvider
		secretStoreClient        internal.SecretStoreClient
		fakeSecretReader         *internalfakes.FakeSecretReader
		credsData                map[string]interface{}
		secretData               map[string]interface{}
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
	)

	Describe("Read Credentials", func() {

		When("the credentials exist in the expected location", func() {
			BeforeEach(func() {
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["username"] = existingRabbitMQUsername
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a credentials provider", func() {
				Expect(credsProvider).NotTo(BeNil())
				Expect(credsProvider.GetUser()).To(Equal(existingRabbitMQUsername))
				Expect(credsProvider.GetPassword()).To(Equal(existingRabbitMQPassword))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("unable to read secret from Vault", func() {
			BeforeEach(func() {
				err = errors.New("something bad happened")
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(nil, err)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to read Vault secret: something bad happened"))
			})
		})

		When("Vault secret data does not contain expected map", func() {
			BeforeEach(func() {
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("returned Vault secret has a nil Data map"))
			})
		})

		When("Vault secret data contains an empty map", func() {
			BeforeEach(func() {
				secretData = make(map[string]interface{})
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("returned Vault secret has an empty Data map"))
			})
		})

		When("Vault secret data map does not contain expected key/value entry", func() {
			BeforeEach(func() {
				secretData = make(map[string]interface{})
				secretData["somekey"] = "somevalue"
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("returned Vault secret has a Data map that contains no value for key 'data'. Available keys are: [somekey]"))
			})
		})

		When("Vault secret data does not contain expected type", func() {
			BeforeEach(func() {
				secretData = make(map[string]interface{})
				secretData["data"] = "I am not a map"
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("data type assertion failed for Vault secret of type: string and value \"I am not a map\" read from path some/path"))
			})
		})

		When("Vault secret data map does not contain username", func() {
			BeforeEach(func() {
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to get username from Vault secret"))
			})
		})

		When("Vault secret data map does not contain password", func() {
			BeforeEach(func() {
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["username"] = existingRabbitMQUsername
				secretData["data"] = credsData
				fakeSecretReader = &internalfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = internal.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				credsProvider, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to get password from Vault secret"))
			})
		})

	})

	Describe("Initialize secret store client", func() {
		var (
			vaultSpec *rabbitmqv1beta1.VaultSpec
		)

		When("vault spec has no role value", func() {
			BeforeEach(func() {
				vaultSpec = &rabbitmqv1beta1.VaultSpec{}
			})

			JustBeforeEach(func() {
				secretStoreClient, err = internal.InitializeSecretStoreClient(vaultSpec)
			})

			PIt("should return a nil secret store client", func() {
				Expect(secretStoreClient).To(BeNil())
			})

			PIt("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("no role value set in Vault secret backend"))
			})
		})

		When("service account token is not in the expected place", func() {
			BeforeEach(func() {
				vaultSpec = &rabbitmqv1beta1.VaultSpec{
					Role: "cheese-and-ham",
				}
			})

			JustBeforeEach(func() {
				secretStoreClient, err = internal.InitializeSecretStoreClient(vaultSpec)
			})

			It("should return a nil secret store client", func() {
				Expect(secretStoreClient).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to read file containing service account token"))
			})
		})

		When("unable to log into vault to obtain client token", func() {
			BeforeEach(func() {
				vaultSpec = &rabbitmqv1beta1.VaultSpec{
					Role: "cheese-and-ham",
				}
				internal.ServiceAccountTokenReader = func() ([]byte, error) {
					return []byte("token"), nil
				}
				internal.VaultAuthenticator = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
					return nil, errors.New("login failed (quickly!)")
				}
			})

			AfterEach(func() {
				internal.ServiceAccountTokenReader = internal.ReadServiceAccountToken
				internal.VaultAuthenticator = internal.LoginToVault
			})

			JustBeforeEach(func() {
				secretStoreClient, err = internal.InitializeSecretStoreClient(vaultSpec)
			})

			It("should return a nil secret store client", func() {
				Expect(secretStoreClient).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to log in with Kubernetes"))
			})
		})

		When("client token obtained from vault", func() {
			BeforeEach(func() {
				vaultSpec = &rabbitmqv1beta1.VaultSpec{
					Role: "cheese-and-ham",
				}
				internal.ServiceAccountTokenReader = func() ([]byte, error) {
					return []byte("token"), nil
				}
				internal.VaultClientTokenReader = func(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (string, error) {
					return "vault-token", nil
				}
			})

			AfterEach(func() {
				internal.ServiceAccountTokenReader = internal.ReadServiceAccountToken
				internal.VaultClientTokenReader = internal.ReadVaultClientToken
			})

			JustBeforeEach(func() {
				secretStoreClient, err = internal.InitializeSecretStoreClient(vaultSpec)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a secret store client", func() {
				Expect(secretStoreClient).ToNot(BeNil())
			})
		})
	})
})
