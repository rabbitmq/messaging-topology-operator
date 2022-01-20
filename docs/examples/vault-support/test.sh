#!/bin/bash

set -euo pipefail

CURRENT_KUBE_NAMESPACE=$(kubectl config view --minify --output 'jsonpath={..namespace}'; echo)

wait_for_operator() {
    sleep 10
    kubectl wait --for=condition=Available deployment/messaging-topology-operator -n rabbitmq-system
    ready="false"
    initialTimeout=30
    timeout=${initialTimeout}
    echo "Waiting for Messaging Topology operator to become ready..."
    # Encountered occasional problems using -q option with grep so falling back to using &>/dev/null
    until kubectl logs --since=10s deployment/messaging-topology-operator -n rabbitmq-system | grep "Starting workers" &>/dev/null && ready="true" || [ "$timeout" -le 0 ]; do
      sleep 5
      timeout=$((timeout - 5))
    done

    if [ "$ready" = "false" ]; then
      echo "Messaging Topology operator not ready after ${initialTimeout} seconds. Exiting..."
      exit 1
    fi
    echo "Messaging Topology operator is ready"
}

# Update the Messaging Topology operator deployment, providing it with the Vault related environment variables
echo "Updating Messaging Topology operator to work with Vault..."
kubectl set env deployment/messaging-topology-operator -n rabbitmq-system --containers='manager' OPERATOR_VAULT_ROLE=messaging-topology-operator VAULT_ADDR=http://vault.$CURRENT_KUBE_NAMESPACE.svc.cluster.local:8200
wait_for_operator

echo
echo "Creating RabbitMQ clusters..."
kubectl apply -f rabbitmq-clusters.yaml
sleep 5
kubectl wait --for=condition=Ready pod/cluster-b-server-0
kubectl wait --for=condition=Ready pod/cluster-vault-a-server-0

echo
echo "Messaging Topology operator attempting to create a queue in each of the RabbitMQ clusters..."
kubectl apply -f rabbitmq-queue-a.yaml
kubectl apply -f rabbitmq-queue-b.yaml
kubectl wait --for=condition=Ready queue.rabbitmq.com/queue-a
kubectl wait --for=condition=Ready queue.rabbitmq.com/queue-b

# Verify that the expected queues exist (one in each cluster)
echo
echo "Listing all queues in RabbitMQ cluster 'cluster-vault-a'..."
kubectl exec cluster-vault-a-server-0 -c rabbitmq -- rabbitmqadmin list queues
echo
echo "Listing all queues in RabbitMQ cluster 'cluster-b'..."
kubectl exec cluster-b-server-0 -c rabbitmq -- rabbitmqadmin list queues

echo
echo "Deleting queues from clusters..."
kubectl delete -f rabbitmq-queue-a.yaml
kubectl delete -f rabbitmq-queue-b.yaml
sleep 5

# Update the Messaging Topology operator deployment, providing it with an intentionally incorrect Vault role identifier
echo
echo "Updating Messaging Topology operator to intentionally break authentication with Vault..."
kubectl set env deployment/messaging-topology-operator -n rabbitmq-system --containers='manager' OPERATOR_VAULT_ROLE=no-such-role
wait_for_operator

# Attempt to create queues in each cluster (creation in the cluster using Vault should fail)
echo
echo "Messaging Topology operator attempting to create a queue in each of the RabbitMQ clusters..."
kubectl apply -f rabbitmq-queue-c.yaml
kubectl apply -f rabbitmq-queue-d.yaml
sleep 5
kubectl wait --for=condition=Ready queue.rabbitmq.com/queue-d

# Verify that only the expected queue in the cluster not using Vault for admin credentials exists
echo
echo "Listing all queues in RabbitMQ cluster 'cluster-vault-a' (should be none)..."
kubectl exec cluster-vault-a-server-0 -c rabbitmq -- rabbitmqadmin list queues
echo
echo "Listing all queues in RabbitMQ cluster 'cluster-b'..."
kubectl exec cluster-b-server-0 -c rabbitmq -- rabbitmqadmin list queues

# Update the Messaging Topology operator deployment to restore correct Vault role identifier before continuing
echo
echo "Updating Messaging Topology operator to repair Vault authentication configuration..."
kubectl set env deployment/messaging-topology-operator -n rabbitmq-system --containers='manager' OPERATOR_VAULT_ROLE=messaging-topology-operator
wait_for_operator

echo
echo "Deleting queues from clusters..."
kubectl delete -f rabbitmq-queue-c.yaml
kubectl delete -f rabbitmq-queue-d.yaml
sleep 5

echo
echo "Deleting RabbitMQ clusters..."
kubectl delete -f rabbitmq-clusters.yaml
sleep 5
kubectl wait --for=delete pod/cluster-vault-a-server-0
kubectl wait --for=delete pod/cluster-b-server-0

echo
echo "Removing Vault env vars from Messaging Topology operator deployment..."
kubectl set env deployment/messaging-topology-operator -n rabbitmq-system --containers='manager' OPERATOR_VAULT_ROLE- VAULT_ADDR-
wait_for_operator

echo
echo "Deleting Vault..."
helm uninstall vault
sleep 5
kubectl wait --for=delete pod/vault-0

echo
echo "Test complete"
