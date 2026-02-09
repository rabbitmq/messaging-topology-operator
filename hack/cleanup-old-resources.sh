#!/bin/bash

# Script to clean up old resources that have been renamed in the refactored operator
# This should be run before upgrading to the new release

set -e

NAMESPACE="rabbitmq-system"

echo "=========================================="
echo "Cleaning up old messaging-topology-operator resources"
echo "Namespace: $NAMESPACE"
echo "=========================================="
echo ""

# Function to delete resource with error handling
delete_resource() {
    local resource_type=$1
    local resource_name=$2
    local scope=${3:-namespaced}
    
    echo "Deleting $resource_type/$resource_name..."
    
    if [ "$scope" = "cluster" ]; then
        if kubectl get "$resource_type" "$resource_name" &>/dev/null; then
            kubectl delete "$resource_type" "$resource_name" --ignore-not-found=true
            echo "  ✓ Deleted cluster-scoped $resource_type/$resource_name"
        else
            echo "  ℹ $resource_type/$resource_name not found (already deleted or never existed)"
        fi
    else
        if kubectl get "$resource_type" "$resource_name" -n "$NAMESPACE" &>/dev/null; then
            kubectl delete "$resource_type" "$resource_name" -n "$NAMESPACE" --ignore-not-found=true
            echo "  ✓ Deleted $resource_type/$resource_name in namespace $NAMESPACE"
        else
            echo "  ℹ $resource_type/$resource_name not found in namespace $NAMESPACE (already deleted or never existed)"
        fi
    fi
}

echo "Step 1: Deleting old ValidatingWebhookConfiguration..."
delete_resource "validatingwebhookconfiguration" "topology.rabbitmq.com" "cluster"
echo ""

echo "Step 2: Deleting old Service..."
delete_resource "service" "webhook-service"
echo ""

echo "Step 3: Deleting old cert-manager resources (if using cert-manager variant)..."
delete_resource "certificate" "serving-cert"
delete_resource "issuer" "selfsigned-issuer"
echo ""

echo "=========================================="
echo "Cleanup complete!"
echo "=========================================="
echo ""
echo "You can now apply the new operator manifests:"
echo "  kubectl apply -f releases/messaging-topology-operator.yaml"
echo "or"
echo "  kubectl apply -f releases/messaging-topology-operator-with-certmanager.yaml"
