// +build tools

package tools

import (
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v3"
)
