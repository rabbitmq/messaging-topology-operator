# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

platform := $(shell uname | tr A-Z a-z)

### Helper functions
### https://stackoverflow.com/questions/10858261/how-to-abort-makefile-if-variable-not-set
check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
        $(error Undefined $1$(if $2, ($2))$(if $(value @), \
                required by target '$@')))
###

##@ General

# runs the target list by default
.DEFAULT_GOAL = list

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
.PHONY: list
list: help ## List Makefile targets (alias for help)

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

LOCAL_TMP := $(CURDIR)/tmp
$(LOCAL_TMP):
	mkdir -p -v $(@)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
YTT ?= $(LOCALBIN)/ytt
CMCTL ?= $(LOCALBIN)/cmctl
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
COUNTERFEITER ?= $(LOCALBIN)/counterfeiter
GINKGO_CLI ?= $(LOCALBIN)/ginkgo
YJ ?= $(LOCALBIN)/yj
GOVULNCHECK ?= $(LOCALBIN)/govulncheck
OPENAPI_GEN ?= $(LOCALBIN)/openapi-gen
KIND ?= $(LOCALBIN)/kind

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.20.0
GOLANGCI_LINT_VERSION ?= v2.7.2
YTT_VERSION ?= v0.50.0
CMCTL_VERSION ?= v2.1.0
CERT_MANAGER_VERSION ?= v1.15.1
CRD_REF_DOCS_VERSION ?= v0.3.0
COUNTERFEITER_VERSION ?= v6.12.1
GINKGO_VERSION ?= v2.27.5
YJ_VERSION ?= v5.1.0
GOVULNCHECK_VERSION ?= v1.1.4
OPENAPI_GEN_VERSION ?= master
KIND_VERSION ?= v0.30.0

# Allows flexibility to use other build kits, like nerdctl
BUILD_KIT ?= docker
CONTAINER_TOOL ?= $(BUILD_KIT)

ARCHITECTURE := $(shell go env GOARCH)
ifeq ($(ARCHITECTURE),aarch64)
	ARCHITECTURE=arm64
endif

# ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

# ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

LOCAL_TESTBIN = $(CURDIR)/testbin

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

##@ Tools

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

.PHONY: counterfeiter
counterfeiter: $(COUNTERFEITER) ## Download counterfeiter locally if necessary.
$(COUNTERFEITER): $(LOCALBIN)
	$(call go-install-tool,$(COUNTERFEITER),github.com/maxbrunsfeld/counterfeiter/v6,$(COUNTERFEITER_VERSION))

.PHONY: ginkgo-cli
ginkgo-cli: $(GINKGO_CLI) ## Download ginkgo CLI locally if necessary.
$(GINKGO_CLI): $(LOCALBIN)
	$(call go-install-tool,$(GINKGO_CLI),github.com/onsi/ginkgo/v2/ginkgo,$(GINKGO_VERSION))

.PHONY: yj
yj: $(YJ) ## Download yj (YAML/JSON converter) locally if necessary.
$(YJ): $(LOCALBIN)
	$(call go-install-tool,$(YJ),github.com/sclevine/yj/v5,$(YJ_VERSION))

.PHONY: govulncheck
govulncheck: $(GOVULNCHECK) ## Download govulncheck locally if necessary.
$(GOVULNCHECK): $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

.PHONY: openapi-gen
openapi-gen: $(OPENAPI_GEN) ## Download openapi-gen locally if necessary.
$(OPENAPI_GEN): $(LOCALBIN)
	$(call go-install-tool,$(OPENAPI_GEN),k8s.io/kube-openapi/cmd/openapi-gen,$(OPENAPI_GEN_VERSION))

.PHONY: kind
kind: $(LOCALBIN) ## Download kind locally if necessary
	@if [ -f $(LOCALBIN)/kind ]; then \
		INSTALLED_VERSION=$$($(LOCALBIN)/kind version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
		if [ "$$INSTALLED_VERSION" != "$(KIND_VERSION)" ]; then \
			echo "Kind version mismatch: installed=$$INSTALLED_VERSION, desired=$(KIND_VERSION). Reinstalling..."; \
			rm -f $(LOCALBIN)/kind; \
		else \
			echo "Kind $(KIND_VERSION) is already installed"; \
			exit 0; \
		fi; \
	fi; \
	echo "Downloading kind $(KIND_VERSION)..."; \
	curl -Lo $(LOCALBIN)/kind https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(platform)-$(ARCHITECTURE); \
	chmod +x $(LOCALBIN)/kind; \
	$(LOCALBIN)/kind version

.PHONY: install-tools
install-tools: controller-gen envtest golangci-lint crd-ref-docs counterfeiter ginkgo-cli yj govulncheck openapi-gen ## Install all tooling required to configure and build this repo
	@echo "All tools installed successfully"

##@ Testing

GINKGO := go run github.com/onsi/ginkgo/v2/ginkgo

# "Control plane binaries (etcd and kube-apiserver) are loaded by default from /usr/local/kubebuilder/bin.
# This can be overridden by setting the KUBEBUILDER_ASSETS environment variable"
# https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
# Note: setup-envtest returns the full path including patch version (e.g., 1.35.0), so we capture it dynamically
KUBEBUILDER_ASSETS_PATH = $(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCAL_TESTBIN) -p path 2>/dev/null || echo "$(LOCAL_TESTBIN)/k8s/$(ENVTEST_K8S_VERSION)-$(platform)-$(ARCHITECTURE)")
export KUBEBUILDER_ASSETS = $(KUBEBUILDER_ASSETS_PATH)

.PHONY: kubebuilder-assets
kubebuilder-assets: $(ENVTEST) ## Download and set up kubebuilder test assets
	@echo "Setting up kubebuilder assets..."
	@mkdir -p $(LOCAL_TESTBIN)
	@path="$$("$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCAL_TESTBIN) -p path)"; \
	if [ -n "$$path" ] && [ -d "$$path" ]; then \
		chmod -R +w "$$path" 2>/dev/null || true; \
		echo "Kubebuilder assets ready at: $$path"; \
		echo "export KUBEBUILDER_ASSETS=$$path"; \
	else \
		echo "Error: Failed to set up kubebuilder assets" >&2; \
		exit 1; \
	fi

.PHONY: clean-testbin
clean-testbin: ## Clean testbin directory (fixes permission issues)
	@echo "Cleaning testbin directory..."
	@if [ -d "$(LOCAL_TESTBIN)" ]; then \
		chmod -R +w $(LOCAL_TESTBIN) 2>/dev/null || true; \
		rm -rf $(LOCAL_TESTBIN); \
		echo "testbin directory cleaned"; \
	else \
		echo "testbin directory does not exist"; \
	fi

.PHONY: unit-tests
unit-tests::controller-gen ## Run unit tests
unit-tests::kubebuilder-assets
unit-tests::generate
unit-tests::fmt
unit-tests::vet
unit-tests::manifests
unit-tests::just-unit-tests

.PHONY: just-unit-tests
just-unit-tests: ## Run just unit tests without regenerating code
	$(GINKGO) -r --randomize-all --label-filter="!controller-suite" api/ internal/ rabbitmqclient/

.PHONY: integration-tests
integration-tests::controller-gen ## Run integration tests. Use GINKGO_EXTRA="-some-arg" to append arguments to 'ginkgo run'
integration-tests::kubebuilder-assets
integration-tests::generate
integration-tests::fmt
integration-tests::vet
integration-tests::manifests
integration-tests::just-integration-tests

.PHONY: just-integration-tests
just-integration-tests: kubebuilder-assets ## Run just integration tests without regenerating code
	$(GINKGO) --randomize-all -r -p $(GINKGO_EXTRA) internal/controller/

.PHONY: local-tests
local-tests: unit-tests integration-tests ## Run all local tests (unit & integration)

SYSTEM_TEST_NS ?= rabbitmq-system
RABBITMQ_SVC_TYPE ?=
.PHONY: system-tests
system-tests: ## Run E2E tests using current context in ~/.kube/config. Expects cluster operator and topology operator to be installed in the cluster
	NAMESPACE="$(SYSTEM_TEST_NS)" RABBITMQ_SVC_TYPE="$(RABBITMQ_SVC_TYPE)" $(GINKGO) --randomize-all -r $(GINKGO_EXTRA) test/system/

KIND_CLUSTER ?= kubebuilder-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: kind ## Set up a Kind cluster for e2e tests if it does not exist
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

##@ Build & Run

.PHONY: manager
manager: generate fmt vet vuln ## Build manager binary
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
#
# Since this runs outside a cluster and there's a requirement on cluster-level service
# communication, the connection between them needs to be accounted for.
# https://github.com/telepresenceio/telepresence is one way to do this (just run
# `telepresence connect` and services like `test-service.test-namespace.svc.cluster.local`
# will resolve properly).
.PHONY: run
run: generate fmt vet vuln manifests install just-run ## Run controller from your host

.PHONY: just-run
just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system ENABLE_WEBHOOKS=false ENABLE_DEBUG_PPROF=true go run ./main.go -metrics-bind-address 127.0.0.1:8080

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | $(KUBECTL) apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | $(KUBECTL) delete --ignore-not-found=true -f -; else echo "No CRDs to delete; skipping."; fi

##@ Development

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate API reference documentation
.PHONY: api-reference
api-reference: crd-ref-docs ## Generate API reference documentation
	"$(CRD_REF_DOCS)" \
		--source-path ./api \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/rabbitmq.com.ref.asciidoc \
		--max-depth 30

# Run go fmt against code
.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

# Run govulncheck
.PHONY: vuln
vuln: govulncheck ## Run govulncheck
	"$(GOVULNCHECK)" ./...

# Generate code & docs
.PHONY: generate
generate: controller-gen api-reference ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations
	"$(CONTROLLER_GEN)" object:headerFile="hack/NOTICE.go.txt" paths="./..."

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

##@ Docker Build

QUAY_IO_OPERATOR_IMAGE ?= quay.io/rabbitmqoperator/messaging-topology-operator:latest
GHCR_IO_OPERATOR_IMAGE ?= ghcr.io/rabbitmq/messaging-topology-operator:latest
IMG ?= $(QUAY_IO_OPERATOR_IMAGE)
GIT_COMMIT=$(shell git rev-parse --short HEAD)-dev
OPERATOR_IMAGE ?= rabbitmqoperator/messaging-topology-operator
GOFIPS140 ?= off

## used in CI pipeline to create release artifact
.PHONY: generate-manifests
generate-manifests: ytt kustomize ## Generate release manifests for distribution
	mkdir -p releases
	"$(KUSTOMIZE)" build config/installation/  > releases/messaging-topology-operator.yaml
	"$(YTT)" -f releases/messaging-topology-operator.yaml -f config/ytt_overlays/change_deployment_image.yml --data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) > releases/messaging-topology-operator-quay-io.yaml
	"$(YTT)" -f releases/messaging-topology-operator.yaml -f config/ytt_overlays/change_deployment_image.yml --data-value operator_image=$(GHCR_IO_OPERATOR_IMAGE) > releases/messaging-topology-operator-ghcr-io.yaml
	cp -v releases/messaging-topology-operator.yaml releases/messaging-topology-operator-with-certmanager.yaml

.PHONY: docker-build
docker-build: ## Build docker image with the manager
	$(CONTAINER_TOOL) build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: docker-build-dev
docker-build-dev: ## Build and push development docker image
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	$(CONTAINER_TOOL) buildx build --build-arg=FIPS_MODE=$(GOFIPS140) --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	$(CONTAINER_TOOL) push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

# docker-build-local and deploy-local work in local Kubernetes installations where the Kubernetes API
# server runs in the same Docker Context as the build process. This is the case for Rancher Desktop
# and probably for Docker Desktop. These two commands won't have the desired effect if Kubernetes API
# is running remotely e.g. Cloud, or if the build context is not shared with Kubernetes API e.g. containerd
.PHONY: docker-build-local
docker-build-local: ## Build docker image locally (for local K8s like Rancher Desktop)
	$(CONTAINER_TOOL) buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t localhost/topology-operator:$(GIT_COMMIT) .

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/myoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name kubebuilder-builder
	$(CONTAINER_TOOL) buildx use kubebuilder-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm kubebuilder-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

##@ Deploy & Teardown

K8S_OPERATOR_NAMESPACE ?= rabbitmq-system

.PHONY: docker-registry-secret
docker-registry-secret: ## Create docker registry secret in K8s cluster
	$(call check_defined, DOCKER_REGISTRY_USERNAME, Username for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_PASSWORD, Password for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_SECRET, Name of Kubernetes secret in which to store the Docker registry username and password)
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	@echo "Creating registry secret and patching default service account"
	@$(KUBECTL) -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) \
		--docker-server='$(DOCKER_REGISTRY_SERVER)' \
		--docker-username="$$DOCKER_REGISTRY_USERNAME" \
		--docker-password="$$DOCKER_REGISTRY_PASSWORD" || true

.PHONY: deploy
deploy: manifests kustomize deploy-manager ## Deploy controller to the K8s cluster specified in ~/.kube/config

.PHONY: deploy-manager
deploy-manager: cmctl ## Deploy operator manager
	"$(CMCTL)" check api --wait=2m
	"$(KUSTOMIZE)" build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config
	"$(KUSTOMIZE)" build config/default | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: destroy
destroy: undeploy ## Delete all resources of this Operator (alias for undeploy)

# Deploy operator with local changes
.PHONY: just-deploy-dev
just-deploy-dev: ytt cmctl kustomize ## Build current code as a Docker image, push the image, and deploy to current Kubernetes context
	$(call check_defined, DOCKER_REGISTRY_USERNAME, Username for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_PASSWORD, Password for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_SECRET, Name of Kubernetes secret in which to store the Docker registry username and password)
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	"$(CMCTL)" check api --wait=2m
	"$(KUSTOMIZE)" build config/default/ \
		| "$(YTT)" -f- -f config/ytt_overlays/change_deployment_image.yml \
			-f config/ytt_overlays/always_pull.yml \
			-f config/ytt_overlays/add_pull_secrets.yml \
			--data-value operator_image="$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)" \
			--data-value pull_secret_name="$(DOCKER_REGISTRY_SECRET)" \
		| $(KUBECTL) apply -f -

.PHONY: deploy-dev
deploy-dev::cmctl ## Deploy development version of operator
deploy-dev::docker-build-dev
deploy-dev::manifests
deploy-dev::just-deploy-dev
deploy-dev::docker-registry-secret

# Load operator image and deploy operator into current KinD cluster
.PHONY: deploy-kind
deploy-kind: manifests cmctl kustomize ytt ## Deploy operator to KinD cluster
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	$(CONTAINER_TOOL) buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	"$(CMCTL)" check api --wait=2m
	"$(KUSTOMIZE)" build config/default | "$(YTT)" -f- -f config/ytt_overlays/change_deployment_image.yml \
		--data-value operator_image="$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)" \
		-f config/ytt_overlays/never_pull.yml | $(KUBECTL) apply -f -

.PHONY: deploy-local
deploy-local: cmctl ytt kustomize ## Deploy operator to local K8s (Rancher Desktop, Docker Desktop)
	"$(CMCTL)" check api --wait=2m
	"$(KUSTOMIZE)" build config/default | "$(YTT)" -f- -f config/ytt_overlays/change_deployment_image.yml \
		--data-value operator_image="localhost/topology-operator:$(GIT_COMMIT)" \
		-f config/ytt_overlays/never_pull.yml | $(KUBECTL) apply -f-

.PHONY: deploy-e2e
deploy-e2e: manifests kustomize ytt ## Deploy operator for e2e tests (uses imagePullPolicy: Never)
	@echo "Deploying operator for e2e tests with image: $(IMG)"
	@cd config/manager && "$(KUSTOMIZE)" edit set image controller=$(IMG)
	@echo "Building operator manifest..."
	@"$(KUSTOMIZE)" build config/default \
		| sed 's/namespace: rabbitmq-system/namespace: messaging-topology-operator-system/g' \
		| "$(YTT)" -f- -f config/ytt_overlays/never_pull.yml \
			-f config/ytt_overlays/skip_namespace.yml \
			-f config/ytt_overlays/fix_rbac_namespace.yml \
			-f config/ytt_overlays/enable_secure_metrics.yml \
		| $(KUBECTL) apply -f -

.PHONY: cluster-operator
cluster-operator: ## Deploy RabbitMQ Cluster Operator
	@$(KUBECTL) apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml

.PHONY: destroy-cluster-operator
destroy-cluster-operator: ## Delete RabbitMQ Cluster Operator
	@$(KUBECTL) delete -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml --ignore-not-found

################
# Cert Manager #
################

# https://github.com/cert-manager/cmctl/releases
# Cert Manager now publishes CMCTL independently from cert-manager
.PHONY: cmctl
cmctl: $(CMCTL) ## Download cmctl locally if necessary
$(CMCTL): $(LOCALBIN) $(LOCAL_TMP)
	curl -sSL -o $(LOCAL_TMP)/cmctl.tar.gz https://github.com/cert-manager/cmctl/releases/download/$(CMCTL_VERSION)/cmctl_$(platform)_$(shell go env GOARCH).tar.gz
	tar -C $(LOCAL_TMP) -xzf $(LOCAL_TMP)/cmctl.tar.gz
	mv $(LOCAL_TMP)/cmctl $(CMCTL)

.PHONY: cert-manager
cert-manager: ## Deploys Cert Manager from JetStack repo. Use CERT_MANAGER_VERSION to customise version e.g. v1.15.1
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

.PHONY: destroy-cert-manager
destroy-cert-manager: ## Deletes Cert Manager deployment created by 'make cert-manager'
	$(KUBECTL) delete -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

##@ YTT

# https://github.com/carvel-dev/ytt/releases
.PHONY: ytt
ytt: $(YTT) ## Download ytt locally if necessary
$(YTT): $(LOCALBIN)
	@[ -f "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)" ] && [ "$$(readlink -- "$(YTT)" 2>/dev/null)" = "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)" ] || { \
		printf "Downloading and installing Carvel YTT\n"; \
		curl -sSL -o "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)" https://github.com/carvel-dev/ytt/releases/download/$(YTT_VERSION)/ytt-$(platform)-$(ARCHITECTURE); \
		chmod +x "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)"; \
		printf "Carvel YTT $(YTT_VERSION) installed locally\n"; \
	}; \
	ln -sf "$$(realpath "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)")" "$(YTT)"

##@ Legacy

.PHONY: generate-client-set
generate-client-set: ## Generate client-set code (legacy)
	$(get_mod_code_generator)
	go mod vendor
	./hack/update-codegen.sh
