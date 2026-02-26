package testutils

import (
	"io"
	"os"
	"path/filepath"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"

	gomega "github.com/onsi/gomega"
)

func CreateCertFile(offset int, fileName string) (string, *os.File) {
	tmpDir, err := os.MkdirTemp("", "certs")
	gomega.ExpectWithOffset(offset, err).ToNot(gomega.HaveOccurred())
	path := filepath.Join(tmpDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0755)
	gomega.ExpectWithOffset(offset, err).ToNot(gomega.HaveOccurred())
	return path, file
}

// GenerateCertandKey generate a pair of certificate and key, given a cacert
func GenerateCertandKey(offset int, hostname string, caCert, caKey []byte, certWriter, keyWriter io.Writer) {
	caPriv, err := helpers.ParsePrivateKeyPEM(caKey)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

	caPub, err := helpers.ParseCertificatePEM(caCert)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

	s, err := local.NewSigner(caPriv, caPub, signer.DefaultSigAlgo(caPriv), nil)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

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
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

	signReq := signer.SignRequest{Hosts: serverReq.Hosts, Request: string(serverCsr)}
	serverCert, err := s.Sign(signReq)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

	_, err = certWriter.Write(serverCert)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())
	_, err = keyWriter.Write(serverKey)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())
}

// creates a CA cert, and uses it to sign another cert
// it returns the generated ca cert and key so they can be reused
func CreateCertificateChain(offset int, hostname string, caCertWriter, certWriter, keyWriter io.Writer) ([]byte, []byte) {
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
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

	_, err = caCertWriter.Write(caCert)
	gomega.ExpectWithOffset(offset, err).NotTo(gomega.HaveOccurred())

	GenerateCertandKey(offset+1, hostname, caCert, caKey, certWriter, keyWriter)

	return caCert, caKey
}
