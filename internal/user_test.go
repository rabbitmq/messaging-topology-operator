package internal_test

import (
	"crypto/sha512"
	"encoding/base64"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GenerateUserSettings", func() {
	var credentialSecret corev1.Secret
	var userTags []topology.UserTag

	BeforeEach(func() {
		credentialSecret = corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("my-rabbit-user"),
				"password": []byte("a-secure-password"),
			},
		}
		userTags = []topology.UserTag{"administrator", "monitoring"}
	})

	It("uses the password to generate the expected rabbithole.UserSettings", func() {
		settings, err := internal.GenerateUserSettings(&credentialSecret, userTags)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Name).To(Equal("my-rabbit-user"))
		Expect(settings.Tags).To(ConsistOf("administrator", "monitoring"))
		Expect(settings.HashingAlgorithm.String()).To(Equal(rabbithole.HashingAlgorithmSHA512.String()))

		// Password should not be sent, even if provided
		Expect(settings.Password).To(BeEmpty())

		// The first 4 bytes of the PasswordHash will be the salt used in the hashing algorithm.
		// See https://www.rabbitmq.com/passwords.html#computing-password-hash.
		// We can take this salt and calculate what the correct hashed salted value would
		// be for our original plaintext password.
		passwordHashBytes, err := base64.StdEncoding.DecodeString(settings.PasswordHash)
		Expect(err).NotTo(HaveOccurred())

		salt := passwordHashBytes[0:4]
		saltedHash := sha512.Sum512([]byte(string(salt) + "a-secure-password"))
		Expect(base64.StdEncoding.EncodeToString([]byte(string(salt) + string(saltedHash[:])))).To(Equal(settings.PasswordHash))
	})

	It("uses the passwordHash to generate the expected rabbithole.UserSettings", func() {
		hash, _ := rabbithole.SaltedPasswordHashSHA256("a-different-password")
		credentialSecret.Data["passwordHash"] = []byte(hash)

		settings, err := internal.GenerateUserSettings(&credentialSecret, userTags)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Name).To(Equal("my-rabbit-user"))
		Expect(settings.Tags).To(ConsistOf("administrator", "monitoring"))
		Expect(settings.HashingAlgorithm.String()).To(Equal(rabbithole.HashingAlgorithmSHA512.String()))
		Expect(settings.PasswordHash).To(Equal(hash))

		// Password should not be sent, even if provided
		Expect(settings.Password).To(BeEmpty())
	})

	When("user limits are provided", func() {
		var connections, channels int32

		It("uses the limits to generate the expected rabbithole.UserLimits", func() {
			connections = 3
			channels = 7
			limits := internal.GenerateUserLimits(&topology.UserLimits{
				Connections: &connections,
				Channels:    &channels,
			})
			Expect(limits).To(HaveLen(2))
			Expect(limits).To(HaveKeyWithValue("max-connections", int(connections)))
			Expect(limits).To(HaveKeyWithValue("max-channels", int(channels)))
		})

		It("does not create unspecified limits", func() {
			connections = 5
			limits := internal.GenerateUserLimits(&topology.UserLimits{
				Connections: &connections,
			})
			Expect(limits).To(HaveKeyWithValue("max-connections", int(connections)))
			Expect(limits).NotTo(HaveKey("max-channels"))
		})
	})

	When("no user limits are provided", func() {
		It("does not specify limits", func() {
			limits := internal.GenerateUserLimits(&topology.UserLimits{})
			Expect(limits).To(BeEmpty())
		})
	})
})
