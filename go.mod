module github.com/rabbitmq/messaging-topology-operator

go 1.16

require (
	github.com/cloudflare/cfssl v1.6.0
	github.com/elastic/crd-ref-docs v0.0.7
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.2.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.4.1
	github.com/michaelklishin/rabbit-hole/v2 v2.10.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/rabbitmq/cluster-operator v1.7.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/code-generator v0.21.2
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/controller-tools v0.6.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
)
