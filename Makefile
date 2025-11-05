SHELL := bash
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

# runs the target list by default
.DEFAULT_GOAL = list

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
list:    ## List Makefile targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

#############
### Tools ###
#############

# Allows flexibility to use other build kits, like nerdctl
BUILD_KIT ?= docker

install-tools: ## Install tooling required to configure and build this repo
	@echo "Install all tools..."
	cd internal/tools; grep _ tools.go | awk -F '"' '{print $$2}' | xargs -t go install -mod=mod

LOCAL_TESTBIN = $(CURDIR)/testbin
ENVTEST_K8S_VERSION = 1.26.1
ARCHITECTURE = $(shell go env GOARCH)

ifeq ($(ARCHITECTURE),aarch64)
	ARCHITECTURE=arm64
endif


LOCAL_BIN := $(CURDIR)/bin
$(LOCAL_BIN):
	mkdir -p -v $(@)

LOCAL_TMP := $(CURDIR)/tmp
$(LOCAL_TMP):
	mkdir -p -v $(@)

# "Control plane binaries (etcd and kube-apiserver) are loaded by default from /usr/local/kubebuilder/bin.
# This can be overridden by setting the KUBEBUILDER_ASSETS environment variable"
# https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
export KUBEBUILDER_ASSETS = $(LOCAL_TESTBIN)/k8s/$(ENVTEST_K8S_VERSION)-$(platform)-$(ARCHITECTURE)

.PHONY: kubebuilder-assets
kubebuilder-assets: $(KUBEBUILDER_ASSETS)
	@echo "export KUBEBUILDER_ASSETS=$(LOCAL_TESTBIN)/k8s/$(ENVTEST_K8S_VERSION)-$(platform)-$(ARCHITECTURE)"

$(KUBEBUILDER_ASSETS):
	setup-envtest --os $(platform) --arch $(ARCHITECTURE) --bin-dir $(LOCAL_TESTBIN) use $(ENVTEST_K8S_VERSION)

# https://github.com/carvel-dev/ytt/releases
YTT_VERSION ?= v0.50.0
YTT = $(LOCAL_BIN)/ytt-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)
.PHONY: ytt
ytt: | $(YTT)
$(YTT): | $(LOCAL_BIN)
	@printf "Downloading and installing Carvel YTT\n"
	@curl -sSL -o $(YTT) https://github.com/carvel-dev/ytt/releases/download/$(YTT_VERSION)/ytt-$(platform)-$(ARCHITECTURE)
	@chmod +x $(YTT)
	@ln -s $(LOCAL_BIN)/ytt-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE) $(LOCAL_BIN)/ytt
	@printf "Carvel YTT $(YTT_VERSION) installed locally\n"

##############
#### Tests ###
##############
GINKGO := go run github.com/onsi/ginkgo/v2/ginkgo

.PHONY: unit-tests
unit-tests::install-tools ## Run unit tests
unit-tests::$(KUBEBUILDER_ASSETS)
unit-tests::generate
unit-tests::fmt
unit-tests::vet
unit-tests::manifests
unit-tests::just-unit-tests

.PHONY: just-unit-tests
just-unit-tests:
	$(GINKGO) -r --randomize-all api/ internal/ rabbitmqclient/

.PHONY: integration-tests
integration-tests::install-tools ## Run integration tests. Use GINKGO_EXTRA="-some-arg" to append arguments to 'ginkgo run'
integration-tests::$(KUBEBUILDER_ASSETS)
integration-tests::generate
integration-tests::fmt
integration-tests::vet
integration-tests::manifests
integration-tests::just-integration-tests

just-integration-tests: $(KUBEBUILDER_ASSETS)
	$(GINKGO) --randomize-all -r -p $(GINKGO_EXTRA) controllers/

.PHONY: local-tests
local-tests: unit-tests integration-tests ## Run all local tests (unit & integration)

SYSTEM_TEST_NS ?= rabbitmq-system
.PHONY: system-tests
system-tests: ## Run E2E tests using current context in ~/.kube/config. Expects cluster operator and topology operator to be installed in the cluster
	NAMESPACE="$(SYSTEM_TEST_NS)" $(GINKGO) --randomize-all -r $(GINKGO_EXTRA) system_tests/


###################
### Build & Run ###
###################
.PHONY: manager
manager: generate fmt vet vuln
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
#
# Since this runs outside a cluster and there's a requirement on cluster-level service
# communication, the connection between them needs to be accounted for.
# https://github.com/telepresenceio/telepresence is one way to do this (just run
# `telepresence connect` and services like `test-service.test-namespace.svc.cluster.local`
# will resolve properly).
.PHONY: run
run: generate fmt vet vuln manifests install just-run

.PHONY: just-run
just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system ENABLE_WEBHOOKS=false ENABLE_DEBUG_PPROF=true go run ./main.go -metrics-bind-address 127.0.0.1:8080

.PHONY: install
install: manifests
	kustomize build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

.PHONY: deploy-manager
deploy-manager: cmctl
	$(CMCTL) check api --wait=2m
	kustomize build config/default/overlays/cert-manager/ | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: install-tools
	controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate API reference documentation
.PHONY: api-reference
api-reference: install-tools
	crd-ref-docs \
		--source-path ./api \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/rabbitmq.com.ref.asciidoc \
		--max-depth 30

QUAY_IO_OPERATOR_IMAGE ?= quay.io/rabbitmqoperator/messaging-topology-operator:latest
GHCR_IO_OPERATOR_IMAGE ?= ghcr.io/rabbitmq/messaging-topology-operator:latest
## used in CI pipeline to create release artifact
.PHONY: generate-manifests
generate-manifests: | $(YTT)
	mkdir -p releases
	kustomize build config/installation/  > releases/messaging-topology-operator.bak
	sed '/CERTIFICATE_NAMESPACE.*CERTIFICATE_NAME/d' releases/messaging-topology-operator.bak > releases/messaging-topology-operator.yaml
	$(YTT) -f releases/messaging-topology-operator.yaml -f config/ytt_overlays/change_deployment_image.yml --data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) > releases/messaging-topology-operator-quay-io.yaml
	$(YTT) -f releases/messaging-topology-operator.yaml -f config/ytt_overlays/change_deployment_image.yml --data-value operator_image=$(GHCR_IO_OPERATOR_IMAGE) > releases/messaging-topology-operator-ghcr-io.yaml
	kustomize build config/installation/cert-manager/ > releases/messaging-topology-operator-with-certmanager.yaml
	$(YTT) -f releases/messaging-topology-operator-with-certmanager.yaml -f config/ytt_overlays/change_deployment_image.yml --data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) > releases/messaging-topology-operator-with-certmanager-quay-io.yaml
	$(YTT) -f releases/messaging-topology-operator-with-certmanager.yaml -f config/ytt_overlays/change_deployment_image.yml --data-value operator_image=$(GHCR_IO_OPERATOR_IMAGE) > releases/messaging-topology-operator-with-certmanager-ghcr-io.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run govulncheck
vuln:
	govulncheck ./...

# Generate code & docs
generate: install-tools api-reference
	controller-gen object:headerFile="hack/NOTICE.go.txt" paths="./..."

.PHONY: generate-client-set
generate-client-set:
	$(get_mod_code_generator)
	go mod vendor
	./hack/update-codegen.sh

GIT_COMMIT=$(shell git rev-parse --short HEAD)-dev
OPERATOR_IMAGE ?= rabbitmqoperator/messaging-topology-operator
GOFIPS140 ?= off
.PHONY: docker-build-dev
docker-build-dev:
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	$(BUILD_KIT) buildx build --build-arg=FIPS_MODE=$(GOFIPS140) --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	$(BUILD_KIT) push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

# docker-build-local and deploy-local work in local Kubernetes installations where the Kubernetes API
# server runs in the same Docker Context as the build process. This is the case for Rancher Desktop
# and probably for Docker Desktop. These two commands won't have the desired effect if Kubernetes API
# is running remotely e.g. Cloud, or if the build context is not shared with Kubernetes API e.g. containerd
.PHONY: docker-build-local
docker-build-local:
	$(BUILD_KIT) buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t localhost/topology-operator:$(GIT_COMMIT) .

K8S_OPERATOR_NAMESPACE ?= rabbitmq-system
.PHONY: docker-registry-secret
docker-registry-secret:
	$(call check_defined, DOCKER_REGISTRY_USERNAME, Username for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_PASSWORD, Password for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_SECRET, Name of Kubernetes secret in which to store the Docker registry username and password)
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	@echo "Creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) \
		--docker-server='$(DOCKER_REGISTRY_SERVER)' \
		--docker-username="$$DOCKER_REGISTRY_USERNAME" \
		--docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount messaging-topology-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

#########################
### Deploy & Teardown ###
#########################
.PHONY: deploy
deploy: manifests deploy-rbac deploy-manager ## Deploy latest version of this Operator

.PHONY: destroy
destroy: ## Delete all resources of this Operator
	kustomize build config/rbac | kubectl delete --ignore-not-found=true -f -
	kustomize build config/default/base | kubectl delete --ignore-not-found=true -f -

# Deploy operator with local changes
.PHONY: deploy-dev
deploy-dev: cmctl docker-build-dev manifests deploy-rbac docker-registry-secret ## Build current code as a Docker image, push the image, and deploy to current Kubernetes context
	$(call check_defined, DOCKER_REGISTRY_USERNAME, Username for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_PASSWORD, Password for accessing the docker registry)
	$(call check_defined, DOCKER_REGISTRY_SECRET, Name of Kubernetes secret in which to store the Docker registry username and password)
	$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image (e.g. registry.my-company.com))
	$(CMCTL) check api --wait=2m
	kustomize build config/default/overlays/dev | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

# Load operator image and deploy operator into current KinD cluster
.PHONY: deploy-kind
deploy-kind: manifests cmctl deploy-rbac
	$(BUILD_KIT) buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	$(CMCTL) check api --wait=2m
	kustomize build config/default/overlays/kind | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

.PHONY: deploy-local
deploy-local: cmctl deploy-rbac $(YTT)
	$(CMCTL) check api --wait=2m
	kustomize build config/default/overlays/cert-manager | $(YTT) -f- -f config/ytt_overlays/change_deployment_image.yml \
		--data-value operator_image="localhost/topology-operator:$(GIT_COMMIT)" \
		-f config/ytt_overlays/never_pull.yml | kubectl apply -f-

.PHONY: deploy-rbac
deploy-rbac:
	kustomize build config/rbac | kubectl apply -f -

.PHONY: cluster-operator
cluster-operator:
	@kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml

.PHONY: destroy-cluster-operator
destroy-cluster-operator:
	@kubectl delete -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml --ignore-not-found

################
# Cert Manager #
################

# https://github.com/cert-manager/cmctl/releases
# Cert Manager now publishes CMCTL independently from cert-manager
CMCTL_VERSION ?= v2.1.0
CMCTL = $(LOCAL_BIN)/cmctl
.PHONY: cmctl
cmctl: | $(CMCTL)
$(CMCTL): | $(LOCAL_BIN) $(LOCAL_TMP)
	curl -sSL -o $(LOCAL_TMP)/cmctl.tar.gz https://github.com/cert-manager/cmctl/releases/download/$(CMCTL_VERSION)/cmctl_$(platform)_$(shell go env GOARCH).tar.gz
	tar -C $(LOCAL_TMP) -xzf $(LOCAL_TMP)/cmctl.tar.gz
	mv $(LOCAL_TMP)/cmctl $(CMCTL)

CERT_MANAGER_VERSION ?= v1.15.1
CERT_MANAGER_MANIFEST ?= https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
.PHONY: cert-manager
cert-manager: ## Deploys Cert Manager from JetStack repo. Use CERT_MANAGER_VERSION to customise version e.g. v1.2.0
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

.PHONY: destroy-cert-manager
destroy-cert-manager: ## Deletes Cert Manager deployment created by 'make cert-manager'
	kubectl delete -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
