SHELL = /bin/sh
PLATFORM := $(shell uname | tr A-Z a-z)
ARCHITECTURE := $(shell uname -m)
ifeq ($(ARCHITECTURE),x86_64)
	ARCHITECTURE=amd64
endif

ifeq ($(ARCHITECTURE),aarch64)
	ARCHITECTURE=arm64
endif

REPO_ROOT := $(shell git rev-parse --show-toplevel)

.DEFAULT_GOAL := all

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

######################
## Bundle manifests ##
######################

.PHONY: all crds rbac deployment webhooks olm-manifests olm-release-config clean

OLM_MANIFEST_DIR = $(REPO_ROOT)/olm/bundle/manifests
$(OLM_MANIFEST_DIR) :
	mkdir -pv $@

TMP_DIR := $(REPO_ROOT)/tmp
$(TMP_DIR) :
	mkdir -pv $@

$(TMP_DIR)/_all_manifests.yaml: $(TMP_DIR)
	kustomize build $(REPO_ROOT)/config/installation/ > $(TMP_DIR)/_all_manifests.yaml

crds: $(TMP_DIR)/_all_manifests.yaml $(TMP_DIR) $(OLM_MANIFEST_DIR)
	yq 'select(.kind == "CustomResourceDefinition")' -s '"rabbitmq.com_" + .spec.names.singular' $(TMP_DIR)/_all_manifests.yaml
	mv -v $(CURDIR)/rabbitmq.com_*.y*ml $(REPO_ROOT)/olm/bundle/manifests/

rbac: $(OLM_TMP_DIR) ## Extract RBAC rules to a temporary file
	yq '{"rules": .rules}' $(REPO_ROOT)/config/rbac/role.yaml > $(TMP_DIR)/role-rules.yaml

QUAY_IO_OPERATOR_IMAGE ?= quay.io/rabbitmqoperator/messaging-topology-operator:latest
deployment: $(TMP_DIR)/_all_manifests.yaml $(TMP_DIR) $(OLM_MANIFEST_DIR) ## Extract Deployment spec. Customise using QUAY_IO_OPERATOR_IMAGE
	kustomize build $(REPO_ROOT)/config/installation/ | \
		ytt -f- -f $(REPO_ROOT)/config/ytt_overlays/change_deployment_image.yml \
			--data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) \
			-f $(REPO_ROOT)/olm/bundle/templates/topology-operator-namespace-scope-overlay.yml \
		> $(TMP_DIR)/topology-operator.yml
	yq 'select(.kind =="Deployment") | {"spec": .spec}' $(TMP_DIR)/topology-operator.yml > $(TMP_DIR)/spec.yaml

webhooks: $(TMP_DIR)/_all_manifests.yaml $(TMP_DIR) ## Extract Web-hook manifests into a temporary file
	yq 'select(.kind =="ValidatingWebhookConfiguration") | {"webhooks": .webhooks}' $(TMP_DIR)/_all_manifests.yaml \
	> $(TMP_DIR)/_webhooks.yaml

BUNDLE_CREATED_AT ?= $(shell date +'%Y-%m-%dT%H:%M:%S')
BUNDLE_VERSION ?= 0.0.0
olm-manifests: $(OLM_MANIFEST_DIR) ## Render bundle manifests. Customise version using BUNDLE_VERSION and BUNDLE_CREATED_AT
	ytt -f $(REPO_ROOT)/olm/bundle/templates/topology-service-version-generator-openshift.yml \
		--data-values-file $(TMP_DIR)/role-rules.yaml \
		--data-values-file $(TMP_DIR)/spec.yaml \
		--data-values-file $(TMP_DIR)/_webhooks.yaml \
		--data-value name="rabbitmq-messaging-topology-operator" \
		--data-value createdAt="$(BUNDLE_CREATED_AT)" \
		--data-value image="$(QUAY_IO_OPERATOR_IMAGE)" \
		--data-value version="$(BUNDLE_VERSION)" \
		--data-value replaces="$(BUNDLE_REPLACES)" \
		> $(OLM_MANIFEST_DIR)/rabbitmq-messaging-topology-operator.clusterserviceversion.yaml

olm-release-config: ## Render release config manifests. Set replaces using BUNDLE_REPLACES
	ytt -f $(REPO_ROOT)/olm/release-config-template.yaml \
		--data-value replaces="$(BUNDLE_REPLACES)" \
		--data-values-file $(REPO_ROOT)/olm/values-release.yaml \
		> $(REPO_ROOT)/olm/release-config.yaml

all::crds ## Default goal. Generates bundle manifests
all::rbac
all::deployment
all::webhooks
all::olm-manifests
all::olm-release-config

clean:
	rm -r -f -v $(REPO_ROOT)/olm/bundle/manifests/
	rm -r -f -v $(REPO_ROOT)/olm/bundle/crds/

###########
## Build ##
###########

CONTAINER ?= docker
REGISTRY ?= quay.io
IMAGE ?= rabbitmqoperator/rabbitmq-for-kubernetes-olm-messaging-topology-operator:latest
BUNDLE_IMAGE = $(REGISTRY)/$(IMAGE)

.PHONY: docker-build docker-push
docker-build: ## Build bundle container. Customise using REGISTRY and IMAGE
	$(CONTAINER) build -t $(BUNDLE_IMAGE) -f $(REPO_ROOT)/olm/bundle/bundle.Dockerfile $(REPO_ROOT)/olm/bundle

docker-push: ## Push bundle container. Customise using REGISTRY and IMAGE
	$(CONTAINER) push $(BUNDLE_IMAGE)

#############
## Catalog ##
#############
# This is used in tests

CATALOG_DIR = $(REPO_ROOT)/olm/catalog/cool-catalog/rabbitmq-messaging-topology-operator
$(CATALOG_DIR):
	mkdir -p $@

CO_CATALOG_DIR = $(REPO_ROOT)/olm/catalog/cool-catalog/rabbitmq-cluster-operator
$(CO_CATALOG_DIR) :
	mkdir -p $@

.PHONY: catalog-replace-bundle catalog-build catalog-push catalog-deploy catalog-undeploy catalog-clean catalog-all

# Cluster Operator related variables
# Cluster Op is required here because it's a dependency of the top-op
CO_BUNDLE_NAME ?= rabbitmq-cluster-operator
CO_BUNDLE_VERSION ?= 2.15.0
CO_BUNDLE_IMAGE ?= quay.io/rabbitmqoperator/rabbitmq-for-kubernetes-olm-cluster-operator:$(CO_BUNDLE_VERSION)
catalog-replace-bundle: $(CATALOG_DIR) $(CO_CATALOG_DIR) ## Generate catalog manifest. Customise using BUNDLE_IMAGE and BUNDLE_VERSION
	ytt -f $(REPO_ROOT)/olm/catalog/templates/catalog-template.yaml \
		--data-value name="rabbitmq-messaging-topology-operator" \
		--data-value image="$(BUNDLE_IMAGE)" \
		--data-value version="$(BUNDLE_VERSION)" \
		> $(CATALOG_DIR)/catalog.yaml
	ytt -f $(REPO_ROOT)/olm/catalog/templates/catalog-co-template.yaml \
		--data-value cluster_operator_name="$(CO_BUNDLE_NAME)" \
		--data-value cluster_operator_version="$(CO_BUNDLE_VERSION)" \
		--data-value cluster_operator_image="$(CO_BUNDLE_IMAGE)" \
    	> $(CO_CATALOG_DIR)/catalog.yaml

CATALOG_IMAGE ?= rabbitmqoperator/test-catalog:latest-topology
catalog-build: ## Build catalog image. Customise using REGISTRY and CATALOG_IMAGE
	$(CONTAINER) build -t $(REGISTRY)/$(CATALOG_IMAGE) --label "quay.expires-after=48h" -f $(REPO_ROOT)/olm/catalog/cool-catalog.Dockerfile $(REPO_ROOT)/olm/catalog

catalog-push: ## Push catalog image. Customise using REGISTRY and CATALOG_IMAGE
	$(CONTAINER) push $(REGISTRY)/$(CATALOG_IMAGE)

catalog-deploy: ## Deploy a catalog source to an existing k8s
	kubectl apply -f $(REPO_ROOT)/olm/assets/operator-group.yaml
	ytt -f $(REPO_ROOT)/olm/assets/catalog-source.yaml --data-value image="$(REGISTRY)/$(CATALOG_IMAGE)" | kubectl apply -f-
	kubectl apply -f $(REPO_ROOT)/olm/assets/subscription.yaml

catalog-undeploy: ## Delete all catalog assets from k8s
	kubectl delete -f $(REPO_ROOT)/olm/assets/subscription.yaml --ignore-not-found
	kubectl delete -f $(REPO_ROOT)/olm/bundle/manifests/ --ignore-not-found
	kubectl delete -f $(REPO_ROOT)/olm/assets/operator-group.yaml --ignore-not-found
	ytt -f $(REPO_ROOT)/olm/assets/catalog-source.yaml --data-value image="$(REGISTRY)/$(CATALOG_IMAGE)" | kubectl delete -f- --ignore-not-found

catalog-clean: ## Delete manifest files for catalog
	rm -v -f $(REPO_ROOT)/olm/catalog/cool-catalog/*.y*ml

catalog-all::catalog-replace-bundle
catalog-all::catalog-build
catalog-all::catalog-push
catalog-all::catalog-deploy
