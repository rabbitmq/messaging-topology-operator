module github.com/rabbitmq/messaging-topology-operator

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/michaelklishin/rabbit-hole/v2 v2.6.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/rabbitmq/cluster-operator v1.4.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.3
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.2
)
