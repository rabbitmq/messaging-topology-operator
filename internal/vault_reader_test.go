package internal_test

import (
	"github.com/hashicorp/vault/api"
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
		vaultReader              internal.VaultReader
		vaultDelegate            *internalfakes.FakeVaultDelegate
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		vaultSpec                *rabbitmqv1beta1.VaultSpec
		secretData               map[string]interface{}

		requestSecretPath = "some/path"

		/*
			secretStoreClient        internal.VaultReader

			credsData                map[string]interface{}

			vaultWarnings            []string

		*/
	)

	Describe("Read Credentials", func() {

		When("the credentials exist in the expected location", func() {
			BeforeEach(func() {

				secretData = make(map[string]interface{})
				secretData["data"] = make(map[string]interface{})
				credentials, _ := secretData["data"].(map[string]interface{})
				credentials["username"] = existingRabbitMQUsername
				credentials["password"] = existingRabbitMQPassword

				vaultDelegate = new(internalfakes.FakeVaultDelegate)

				vaultSpec = &rabbitmqv1beta1.VaultSpec{

				}
				// Override two collaborators: VaultDelegate and ServiceAccountToken provider
				internal.VaultDelegateFactory = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultDelegate, error) {
					return vaultDelegate, nil
				}
				internal.ServiceAccountToken = func() ([]byte, error) {
					return []byte("some-k8s-token"), nil
				}

				// Expect to successfully authenticate to Vault server
				vaultDelegate.AuthenticateStub = func(m map[string]interface{}) (*api.Secret, error) {
					Expect(m).NotTo(BeNil())
					Expect(m).To(HaveKeyWithValue("jwt", "some-k8s-token"))
					Expect(m).To(HaveKeyWithValue("role", "messaging-topology-operator"))
					return &api.Secret{}, nil
				}
				// Expect to successfully retrieve a secret for the requested path
				vaultDelegate.ReadSecretStub = func(path string) (*api.Secret, error) {
					Expect(path).To(Equal(requestSecretPath))
					return &api.Secret{Data: secretData}, nil
				}

				vaultReader, err = internal.GetVaultReader(vaultSpec)

			})

			JustBeforeEach(func() {
				credsProvider, err = vaultReader.ReadCredentials(requestSecretPath)
			})

			FIt("should return a credentials provider", func() {
				Expect(credsProvider).NotTo(BeNil())
				usernameBytes, _ := credsProvider.Data("username")
				passwordBytes, _ := credsProvider.Data("password")
				Expect(usernameBytes).To(Equal([]byte(existingRabbitMQUsername)))
				Expect(passwordBytes).To(Equal([]byte(existingRabbitMQPassword)))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
		/*
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
					Expect(err.Error()).To(ContainSubstring("returned Vault secret has a nil Data map"))
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

			When("Vault secret data is nil", func() {
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
					Expect(err.Error()).To(ContainSubstring("returned Vault secret has a nil Data map"))
				})
			})

			When("Vault secret is nil", func() {
				BeforeEach(func() {
					fakeSecretReader = &internalfakes.FakeSecretReader{}
					fakeSecretReader.ReadSecretReturns(nil, nil)
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
					Expect(err.Error()).To(ContainSubstring("returned Vault secret is nil"))
				})
			})

			When("Vault secret contains warnings", func() {
				BeforeEach(func() {
					vaultWarnings = append(vaultWarnings, "something bad happened")
					credsData = make(map[string]interface{})
					secretData = make(map[string]interface{})
					credsData["password"] = existingRabbitMQPassword
					secretData["data"] = credsData
					fakeSecretReader = &internalfakes.FakeSecretReader{}
					fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData, Warnings: vaultWarnings}, nil)
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
					Expect(err.Error()).To(ContainSubstring("warnings were returned from Vault"))
					Expect(err.Error()).To(ContainSubstring("something bad happened"))
				})
			})
		*/
	})
	/*
		Describe("Initialize secret store client", func() {
			var (
				vaultSpec                  *rabbitmqv1beta1.VaultSpec
				getSecretStoreClientTester func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultReader, error)
			)

			When("vault spec has no role value", func() {
				BeforeEach(func() {
					vaultSpec = &rabbitmqv1beta1.VaultSpec{}

					getSecretStoreClientTester = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultReader, error) {
						internal.InitializeVaultReader(vaultSpec)()
						return internal.SecretClient, internal.SecretClientCreationError
					}
				})

				JustBeforeEach(func() {
					secretStoreClient, err = getSecretStoreClientTester(vaultSpec)
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
					internal.FirstLoginAttemptResultCh = make(chan error, 1)
					internal.SecretClient = nil
					internal.SecretClientCreationError = nil
					vaultSpec = &rabbitmqv1beta1.VaultSpec{
						Role: "cheese-and-ham",
					}
					getSecretStoreClientTester = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultReader, error) {
						internal.InitializeVaultReader(vaultSpec)()
						return internal.SecretClient, internal.SecretClientCreationError
					}
				})

				JustBeforeEach(func() {
					secretStoreClient, err = getSecretStoreClientTester(vaultSpec)
				})

				It("should return a nil secret store client", func() {
					Expect(secretStoreClient).To(BeNil())
				})

				It("should have returned an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to read file containing service account token"))
				})
			})

			When("unable to log into vault to obtain client secret", func() {
				BeforeEach(func() {
					internal.FirstLoginAttemptResultCh = make(chan error, 1)
					internal.SecretClient = nil
					internal.SecretClientCreationError = nil
					vaultSpec = &rabbitmqv1beta1.VaultSpec{
						Role: "cheese-and-ham",
					}
					internal.ReadServiceAccountTokenFunc = func() ([]byte, error) {
						return []byte("token"), nil
					}
					internal.LoginToVaultFunc = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
						return nil, errors.New("login failed (quickly!)")
					}
					getSecretStoreClientTester = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultReader, error) {
						internal.InitializeVaultReader(vaultSpec)()
						return internal.SecretClient, internal.SecretClientCreationError
					}
				})

				AfterEach(func() {
					internal.ReadServiceAccountTokenFunc = internal.ReadServiceAccountToken
					internal.LoginToVaultFunc = internal.LoginToVault
				})

				JustBeforeEach(func() {
					secretStoreClient, err = getSecretStoreClientTester(vaultSpec)
				})

				It("should return a nil secret store client", func() {
					Expect(secretStoreClient).To(BeNil())
				})

				It("should have returned an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to obtain Vault client secret"))
				})
			})

			When("client secret obtained from vault", func() {
				BeforeEach(func() {
					internal.FirstLoginAttemptResultCh = make(chan error, 1)
					internal.SecretClient = nil
					internal.SecretClientCreationError = nil
					vaultSpec = &rabbitmqv1beta1.VaultSpec{
						Role: "cheese-and-ham",
					}
					internal.ReadServiceAccountTokenFunc = func() ([]byte, error) {
						return []byte("token"), nil
					}
					internal.LoginToVaultFunc = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
						return &vault.Secret{
							Auth: &vault.SecretAuth{
								ClientToken: "vault-secret-token",
							},
						}, nil
					}
					internal.ReadVaultClientSecretFunc = func(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (*vault.Secret, error) {
						return &vault.Secret{
							Auth: &vault.SecretAuth{
								ClientToken: "vault-secret-token",
							},
						}, nil
					}
					getSecretStoreClientTester = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultReader, error) {
						internal.InitializeVaultReader(vaultSpec)()
						return internal.SecretClient, internal.SecretClientCreationError
					}
				})

				AfterEach(func() {
					internal.ReadServiceAccountTokenFunc = internal.ReadServiceAccountToken
					internal.LoginToVaultFunc = internal.LoginToVault
					internal.ReadVaultClientSecretFunc = internal.ReadVaultClientSecret
				})

				JustBeforeEach(func() {
					secretStoreClient, err = getSecretStoreClientTester(vaultSpec)
				})

				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return a secret store client", func() {
					Expect(secretStoreClient).ToNot(BeNil())
				})
			})
		})

	*/
})
