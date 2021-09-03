// +build tools

package tools

import (
	_ "github.com/elastic/crd-ref-docs"
	_ "github.com/maxbrunsfeld/counterfeiter/v6"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v4"

	// These are required for the generated clients.
	_ "k8s.io/client-go/discovery/fake"
	_ "k8s.io/code-generator"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
