SHELL := bash
platform := $(shell uname | tr A-Z a-z)

# runs the target list by default
.DEFAULT_GOAL = list

CRD_OPTIONS ?= "crd:trivialVersions=true, preserveUnknownFields=false"

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
list:    ## list Makefile targets
	@echo "The most used targets: \n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-tools:
	go mod download
	grep _ tools/tools.go | awk -F '"' '{print $$2}' | grep -v k8s.io/code-generator | xargs -t go install
	# This one just needs to be fetched and not installed, get & mod so it ends up in the right place.
	# Note we grep it out above, and just do a go get & go mod for it.
	go get -d k8s.io/code-generator

ENVTEST_K8S_VERSION = 1.20.2
ARCHITECTURE = amd64
LOCAL_TESTBIN = $(CURDIR)/testbin
# "Control plane binaries (etcd and kube-apiserver) are loaded by default from /usr/local/kubebuilder/bin.
# This can be overridden by setting the KUBEBUILDER_ASSETS environment variable"
# https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
export KUBEBUILDER_ASSETS = $(LOCAL_TESTBIN)/k8s/$(ENVTEST_K8S_VERSION)-$(platform)-$(ARCHITECTURE)

$(KUBEBUILDER_ASSETS):
	setup-envtest --os $(platform) --arch $(ARCHITECTURE) --bin-dir $(LOCAL_TESTBIN) use $(ENVTEST_K8S_VERSION)

.PHONY: unit-tests
unit-tests: install-tools $(KUBEBUILDER_ASSETS) generate fmt vet manifests ## Run unit tests
	ginkgo -r --randomizeAllSpecs -p api/ internal/

.PHONY: integration-tests
integration-tests: install-tools $(KUBEBUILDER_ASSETS) generate fmt vet manifests ## Run integration tests
	ginkgo -r --randomizeAllSpecs controllers/

local-tests: unit-tests integration-tests ## Run all local tests (unit & integration)

system-tests: ## run end-to-end tests against Kubernetes cluster defined in ~/.kube/config. Expects cluster operator and messaging topology operator to be installed in the cluster
	NAMESPACE="rabbitmq-system" ginkgo -randomizeAllSpecs -r system_tests/

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
#
# Since this runs outside a cluster and there's a requirement on cluster-level service
# communication, the connection between them needs to be accounted for.
# https://github.com/telepresenceio/telepresence is one way to do this (just run
# `telepresence connect` and services like `test-service.test-namespace.svc.cluster.local`
# will resolve properly).
run: generate fmt vet manifests just-run

just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system ENABLE_WEBHOOKS=false go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

deploy-manager:
	kustomize build config/default/overlays/cert-manager/ | kubectl apply -f -

deploy: manifests deploy-rbac deploy-manager

destroy:
	kustomize build config/rbac | kubectl delete --ignore-not-found=true -f -
	kustomize build config/default/base | kubectl delete --ignore-not-found=true -f -

# Deploy operator with local changes
deploy-dev: check-env-docker-credentials docker-build-dev manifests deploy-rbac docker-registry-secret set-operator-image-repo
	kustomize build config/default/overlays/dev | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

# Load operator image and deploy operator into current KinD cluster
deploy-kind: manifests deploy-rbac
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	kustomize build config/default/overlays/kind | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

deploy-rbac:
	kustomize build config/rbac | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: install-tools
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate API reference documentation
api-reference:
	crd-ref-docs \
		--source-path ./api/v1beta1 \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/rabbitmq.com.ref.asciidoc \
		--max-depth 30

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code & docs
generate: install-tools api-reference
	controller-gen object:headerFile="hack/NOTICE.go.txt" paths="./..."

generate-client-set:
	# This one just needs to be fetched and not installed, get & mod so it ends up in the right place.
	# Note we grep it out above, and just do a go get & go mod for it.
	go get -d k8s.io/code-generator
	go mod vendor
	./hack/update-codegen.sh

check-env-docker-credentials: check-env-registry-server
ifndef DOCKER_REGISTRY_USERNAME
	$(error DOCKER_REGISTRY_USERNAME is undefined: Username for accessing the docker registry)
endif
ifndef DOCKER_REGISTRY_PASSWORD
	$(error DOCKER_REGISTRY_PASSWORD is undefined: Password for accessing the docker registry)
endif
ifndef DOCKER_REGISTRY_SECRET
	$(error DOCKER_REGISTRY_SECRET is undefined: Name of Kubernetes secret in which to store the Docker registry username and password)
endif

docker-build-dev: check-env-docker-repo  git-commit-sha
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

docker-registry-secret: check-env-docker-credentials operator-namespace
	echo "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount messaging-topology-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

git-commit-sha:
ifeq ("", git diff --stat)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
else
GIT_COMMIT=$(shell git rev-parse --short HEAD)-
endif

check-env-registry-server:
ifndef DOCKER_REGISTRY_SERVER
	$(error DOCKER_REGISTRY_SERVER is undefined: URL of docker registry containing the Operator image (e.g. registry.my-company.com))
endif

check-env-docker-repo: check-env-registry-server set-operator-image-repo

set-operator-image-repo:
OPERATOR_IMAGE?=p-rabbitmq-for-kubernetes/messaging-topology-operator

operator-namespace:
ifeq (, $(K8S_OPERATOR_NAMESPACE))
K8S_OPERATOR_NAMESPACE=rabbitmq-system
endif

cluster-operator:
	@kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml

## used in CI pipeline to create release artifact
generate-manifests:
	mkdir -p releases
	kustomize build config/installation/  > releases/messaging-topology-operator.bak
	sed '/CERTIFICATE_NAMESPACE.*CERTIFICATE_NAME/d' releases/messaging-topology-operator.bak > releases/messaging-topology-operator.yaml
	kustomize build config/installation/cert-manager/ > releases/messaging-topology-operator-with-certmanager.yaml

CERT_MANAGER_VERSION ?=v1.2.0
cert-manager:
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

destroy-cert-manager:
	kubectl delete -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
