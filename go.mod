module github.com/rabbitmq/messaging-topology-operator

go 1.16

require (
	github.com/elastic/crd-ref-docs v0.0.7
	github.com/go-logr/logr v0.4.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.4.1
	github.com/michaelklishin/rabbit-hole/v2 v2.7.0
	github.com/onsi/ginkgo v1.16.0
	github.com/onsi/gomega v1.11.0
	github.com/rabbitmq/cluster-operator v1.5.0
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
	k8s.io/code-generator v0.20.5
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/controller-tools v0.5.0
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
)
