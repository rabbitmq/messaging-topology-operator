package internal_test

import (
	"fmt"
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
		vaultReader              internal.VaultReader
		vaultDelegate            *internalfakes.FakeVaultDelegate
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		fullCredentials          map[string]interface{}
		requestSecretPath        = "some/path"
	)

	Describe("Read Credentials", func() {
		var (
			credsProvider internal.CredentialsProvider
		)

		BeforeEach(func() {
			fullCredentials = make(map[string]interface{})
			fullCredentials["data"] = make(map[string]interface{})
			usernamePassword, _ := fullCredentials["data"].(map[string]interface{})
			usernamePassword["username"] = existingRabbitMQUsername
			usernamePassword["password"] = existingRabbitMQPassword

			// and given we are using this vaultDelegate and serviceAccountToken, it should ...
			vaultDelegate = new(internalfakes.FakeVaultDelegate)
			internal.VaultDelegateFactory = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultDelegate, error) {
				return vaultDelegate, nil
			}

			// .... retrieve a k8s service account token
			internal.ServiceAccountToken = func() ([]byte, error) {
				return []byte("some-k8s-token"), nil
			}

			// should authenticate with Vault
			vaultDelegate.AuthenticateStub = func(m map[string]interface{}) (*api.Secret, error) {
				Expect(m).NotTo(BeNil())
				Expect(m).To(HaveKeyWithValue("jwt", "some-k8s-token"))
				Expect(m).To(HaveKeyWithValue("role", "messaging-topology-operator"))
				return &api.Secret{}, nil
			}

		})

		JustBeforeEach(func() {
			vaultReader, err = internal.NewVaultReader(&rabbitmqv1beta1.VaultSpec{})
			credsProvider, err = vaultReader.ReadCredentials(requestSecretPath)
		})

		When("the credentials exist in the expected location", func() {
			BeforeEach(func() {
				//  and should read from Vault a secret for a requestedSecretPath
				vaultDelegate.ReadSecretStub = func(path string) (*api.Secret, error) {
					Expect(path).To(Equal(requestSecretPath))
					return &api.Secret{Data: fullCredentials}, nil
				}

			})

			It("should return a credentials provider", func() {
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

		When("unable to read secret from Vault", func() {
			BeforeEach(func() {
				err = fmt.Errorf("something bad happened")
				vaultDelegate.ReadSecretReturns(nil, err)
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
				vaultDelegate.ReadSecretReturns(&api.Secret{}, nil)
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
				vaultDelegate.ReadSecretReturns(&api.Secret{Data: make(map[string]interface{})}, nil)
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
				secretData := make(map[string]interface{})
				secretData["somekey"] = "somevalue"
				vaultDelegate.ReadSecretReturns(&api.Secret{Data: secretData}, nil)
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
				secretData := make(map[string]interface{})
				secretData["data"] = "I am not a map"
				vaultDelegate.ReadSecretReturns(&api.Secret{Data: secretData}, nil)
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
				credsData := make(map[string]interface{})
				secretData := make(map[string]interface{})
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				vaultDelegate.ReadSecretReturns(&api.Secret{Data: secretData}, nil)
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
				credsData := make(map[string]interface{})
				secretData := make(map[string]interface{})
				credsData["username"] = existingRabbitMQUsername
				secretData["data"] = credsData
				vaultDelegate.ReadSecretReturns(&api.Secret{Data: secretData}, nil)
			})

			It("should return a nil credentials provider", func() {
				Expect(credsProvider).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to get password from Vault secret"))
			})
		})

		When("Vault secret is nil", func() {
			BeforeEach(func() {
				vaultDelegate.ReadSecretReturns(nil, nil)
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
				vaultWarnings := append([]string{}, "something bad happened")
				credsData := make(map[string]interface{})
				secretData := make(map[string]interface{})
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				vaultDelegate.ReadSecretReturns(&api.Secret{Data: secretData, Warnings: vaultWarnings}, nil)
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

	})

	Describe("Initialize VaultReader", func() {
		BeforeEach(func() {
			// and given we are using this vaultDelegate and serviceAccountToken, it should ...
			vaultDelegate = new(internalfakes.FakeVaultDelegate)
			internal.VaultDelegateFactory = func(vaultSpec *rabbitmqv1beta1.VaultSpec) (internal.VaultDelegate, error) {
				return vaultDelegate, nil
			}

			// should authenticate with Vault
			vaultDelegate.AuthenticateStub = func(m map[string]interface{}) (*api.Secret, error) {
				Expect(m).NotTo(BeNil())
				Expect(m).To(HaveKeyWithValue("jwt", "some-k8s-token"))
				Expect(m).To(HaveKeyWithValue("role", "messaging-topology-operator"))
				return &api.Secret{}, nil
			}

			// .... retrieve a k8s service account token
			internal.ServiceAccountToken = func() ([]byte, error) {
				return []byte("some-k8s-token"), nil
			}
		})

		JustBeforeEach(func() {
			vaultReader, err = internal.NewVaultReader(&rabbitmqv1beta1.VaultSpec{})
		})

		When("service account token is not in the expected place", func() {

			BeforeEach(func() {
				internal.ServiceAccountToken = func() ([]byte, error) {
					return nil, fmt.Errorf("error occurred")
				}
			})

			It("should return a nil vault reader", func() {
				Expect(vaultReader).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to read file containing service account token"))
			})
		})

		When("unable to log into vault to obtain client secret", func() {
			BeforeEach(func() {
				vaultDelegate.AuthenticateReturns(nil, fmt.Errorf("failed to login"))
			})

			It("should return a nil vault reader", func() {
				Expect(vaultReader).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to login to Vault"))
			})
		})
	})

})
