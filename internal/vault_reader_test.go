package internal_test

import (
	"errors"

	vault "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
				Expect(err).To(MatchError("data type assertion failed for Vault secret: string \"I am not a map\""))
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

})
