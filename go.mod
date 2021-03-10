module github.com/rabbitmq/messaging-topology-operator

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/michaelklishin/rabbit-hole/v2 v2.6.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.11.0
	github.com/rabbitmq/cluster-operator v1.5.0
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/controller-tools v0.5.0
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
)
