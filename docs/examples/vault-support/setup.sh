#!/bin/bash

set -euo pipefail

RABBITMQ_NAMESPACE=${RABBITMQ_NAMESPACE:-'default'}

vault_exec () {
    kubectl exec vault-0 -c vault -- /bin/sh -c "$*"
}

echo "Installing Vault server and Vault agent injector..."
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update
# For OpenShift deployments, also set the following:
# --set "global.openshift=true"
helm install vault hashicorp/vault \
    --set='server.dev.enabled=true' \
    --set='server.logLevel=debug' \
    --set='injector.logLevel=debug' \
    --wait
sleep 5
kubectl wait --for=condition=Ready pod/vault-0

echo "Configuring K8s authentication..."
# Required so that Vault init container and sidecar of RabbitmqCluster can authenticate with Vault.
vault_exec "vault auth enable kubernetes"

# In Kubernetes 1.21+ clusters, issuer may need to be configured as described in https://www.vaultproject.io/docs/auth/kubernetes#discovering-the-service-account-issuer
# Otherwise, vault-agent-init container will output "error authenticating".
issuer=$(kubectl get --raw=http://127.0.0.1:8001/.well-known/openid-configuration | jq -r .issuer)
vault_exec "vault write auth/kubernetes/config issuer=\"$issuer\" token_reviewer_jwt=\"\$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" kubernetes_host=https://\${KUBERNETES_PORT_443_TCP_ADDR}:443 kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
#vault_exec "vault write auth/kubernetes/config token_reviewer_jwt=\"\$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" kubernetes_host=https://\${KUBERNETES_PORT_443_TCP_ADDR}:443 kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

# Each RabbitMQ cluster may have its own secret path.
echo "Creating credentials for rabbitmq default user in cluster-vault-a..."
vault_exec "vault kv put secret/rabbitmq/cluster-vault-a/creds username='rabbitmq' password='pwda'"
#echo "Creating credentials for rabbitmq default user in cluster-vault-b..."
#vault_exec "vault kv put secret/rabbitmq/cluster-vault-b/creds username='rabbitmq' password='pwdb'"

# Create a policy that allows reading of the default user credentials for the RabbitMQ cluster cluster-vault-a
# The path must be referenced from the RabbitmqCluster cluster-vault-a CRD spec.secretBackend.vault.defaultUserPath
echo "Creating Vault policy named cluster-vault-a-policy for reading of cluster-vault-a credentials..."
vault_exec "vault policy write cluster-vault-a-policy - <<EOF
path \"secret/data/rabbitmq/cluster-vault-a/creds\" {
    capabilities = [\"read\"]
}
EOF
"

# Create a separate policy that allows reading of the default user credentials for the RabbitMQ cluster cluster-vault-b
# The path must be referenced from the RabbitmqCluster cluster-vault-b CRD spec.secretBackend.vault.defaultUserPath
#echo "Creating Vault policy named cluster-vault-b-policy for reading of cluster-vault-b credentials..."
#vault_exec "vault policy write cluster-vault-b-policy - <<EOF
#path \"secret/data/rabbitmq/cluster-vault-b/creds\" {
#    capabilities = [\"read\"]
#}
#EOF
#"

# Define a Vault role that needs to be referenced from the RabbitmqCluster cluster-vault-a and cluster-vault-b CRD spec.secretBackend.vault.role
# bound_service_account_names values follow the pattern "<RabbitmqCluster name>-server”.
# bound_service_account_namespaces values must include rabbitmq-system (where the messaging topology operator is deployed) and the namespaces where the RabbitmqClusters are deployed.
#vault_exec "vault write auth/kubernetes/role/messaging-topology-operator bound_service_account_names=cluster-vault-a-server,cluster-vault-b-server,vault-tls-server,messaging-topology-operator bound_service_account_namespaces=$RABBITMQ_NAMESPACE,rabbitmq-system policies=cluster-vault-a-policy,cluster-vault-b-policy ttl=24h"
vault_exec "vault write auth/kubernetes/role/messaging-topology-operator bound_service_account_names=cluster-vault-a-server,vault-tls-server,messaging-topology-operator bound_service_account_namespaces=$RABBITMQ_NAMESPACE,rabbitmq-system policies=cluster-vault-a-policy ttl=24h"

