# Upgrading to Messaging Topology Operator v1.19.0

## Overview

This guide is for users upgrading from **Messaging Topology Operator v1.18.x or earlier** to **v1.19.0**.

Version 1.19.0 introduces significant structural changes due to a migration to Kubebuilder v4. As part of this refactor, several Kubernetes resources have been renamed to follow Kubebuilder naming conventions, which require breaking changes during the upgrade process.

**Important**: This upgrade requires manual cleanup of old resources before applying the new manifests to avoid conflicts.

## What's Changed

### Resource Name Changes

The following resources have been renamed to follow Kubebuilder v4 conventions (prefixed with `messaging-topology-`):

| Resource Type | Old Name (≤v1.18.x) | New Name (v1.19.0) |
|--------------|-------------------|-------------------|
| Service | `webhook-service` | `messaging-topology-webhook-service` |
| ValidatingWebhookConfiguration | `topology.rabbitmq.com` | `messaging-topology-validating-webhook-configuration` |
| Certificate (cert-manager variant) | `serving-cert` | `messaging-topology-serving-cert` |
| Issuer (cert-manager variant) | `selfsigned-issuer` | `messaging-topology-selfsigned-issuer` |

### New Resources Added

Version 1.19.0 introduces secure metrics with HTTPS and authentication/authorization:

- **Service**: `messaging-topology-controller-metrics-service` - Exposes metrics on port 8443 with HTTPS
- **Certificate**: `messaging-topology-metrics-certs` - Certificate for metrics server (cert-manager variant)
- **ClusterRole**: `messaging-topology-metrics-reader` - Read-only access to metrics endpoint
- **ClusterRole**: `messaging-topology-metrics-auth-role` - Allows the operator to perform token reviews
- **ClusterRoleBinding**: `messaging-topology-metrics-auth-rolebinding` - Binds auth role to operator ServiceAccount

## Upgrade Process

### Step 1: Clean Up Old Resources

Before applying the new manifests, you must remove the old resources that have been renamed.

#### Download the Cleanup Script

Download the cleanup script from the v1.19.0 release:

```bash
curl -LO https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.19.0/cleanup-old-resources.sh
chmod +x cleanup-old-resources.sh
```

#### Run the Cleanup Script

Execute the script to delete old resources:

```bash
./cleanup-old-resources.sh
```

The script will delete the following resources from the `rabbitmq-system` namespace:

- ValidatingWebhookConfiguration: `topology.rabbitmq.com` (cluster-scoped)
- Service: `webhook-service`
- Certificate: `serving-cert` (if using cert-manager variant)
- Issuer: `selfsigned-issuer` (if using cert-manager variant)

The script is safe to run and will gracefully handle resources that don't exist.

### Step 2: Apply New Manifests

Apply the new operator manifests for v1.19.0:

**With cert-manager (recommended):**

```bash
kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.19.0/messaging-topology-operator-with-certmanager.yaml
```

**Without cert-manager:**

```bash
kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.19.0/messaging-topology-operator.yaml
```

### Step 3: Verify the Deployment

Verify that the operator is running correctly:

```bash
kubectl get pods -n rabbitmq-system
kubectl logs -n rabbitmq-system deployment/messaging-topology-operator -c manager -f
```

Expected output should show the operator running without errors.

## Secure Metrics (New Feature)

Version 1.19.0 introduces secure metrics served over HTTPS with Kubernetes authentication and authorization.

### Overview

- **Endpoint**: `https://messaging-topology-controller-metrics-service.rabbitmq-system.svc:8443/metrics`
- **Protocol**: HTTPS (port 8443)
- **Authentication**: Kubernetes bearer token authentication
- **Authorization**: Kubernetes RBAC-based authorization

### Accessing Metrics

To access the metrics endpoint, you need to create a ServiceAccount with appropriate RBAC permissions.

#### 1. Create a ServiceAccount

Create a ServiceAccount in your monitoring namespace:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metrics-reader
  namespace: monitoring
```

#### 2. Bind to the Metrics Reader ClusterRole

The operator provides a `messaging-topology-metrics-reader` ClusterRole. Bind your ServiceAccount to it:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metrics-reader-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: messaging-topology-metrics-reader
subjects:
- kind: ServiceAccount
  name: metrics-reader
  namespace: monitoring
```

Apply these resources:

```bash
kubectl apply -f serviceaccount.yaml
kubectl apply -f clusterrolebinding.yaml
```

#### 3. Access Metrics with curl

From within the cluster, you can access metrics using a bearer token:

```bash
# Generate a token for the ServiceAccount
TOKEN=$(kubectl create token metrics-reader -n monitoring)

# Access metrics from a pod with curl
kubectl run curl --image=curlimages/curl:latest -i --rm --restart=Never -- \
  curl -k -H "Authorization: Bearer $TOKEN" \
  https://messaging-topology-controller-metrics-service.rabbitmq-system.svc:8443/metrics
```

### Prometheus Integration Example

If you're using Prometheus Operator, create a ServiceMonitor to scrape metrics:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: messaging-topology-operator
  namespace: monitoring
  labels:
    app.kubernetes.io/name: messaging-topology-operator
spec:
  endpoints:
  - port: https
    scheme: https
    path: /metrics
    bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    tlsConfig:
      # Use the CA from the ServiceAccount token mount
      insecureSkipVerify: true
  namespaceSelector:
    matchNames:
    - rabbitmq-system
  selector:
    matchLabels:
      app.kubernetes.io/name: messaging-topology-operator
      control-plane: controller-manager
```

**Note**: In production, instead of `insecureSkipVerify: true`, configure proper CA validation using the cert-manager generated certificate.

### Alternative: Access Metrics via kubectl port-forward

For development or debugging, you can use port-forwarding:

```bash
# Port-forward the metrics service
kubectl port-forward -n rabbitmq-system svc/messaging-topology-controller-metrics-service 8443:8443

# In another terminal, access metrics with authentication
TOKEN=$(kubectl create token metrics-reader -n monitoring)
curl -k -H "Authorization: Bearer $TOKEN" https://localhost:8443/metrics
```

## Troubleshooting

### Old Resources Still Exist

If you see conflicts or the upgrade fails, ensure you ran the cleanup script before applying new manifests:

```bash
# Check for old resources
kubectl get validatingwebhookconfiguration topology.rabbitmq.com
kubectl get service webhook-service -n rabbitmq-system

# If they exist, run the cleanup script again
./cleanup-old-resources.sh
```

### Metrics Endpoint Returns 403 Forbidden

This indicates your ServiceAccount doesn't have the correct RBAC permissions:

1. Verify the ClusterRoleBinding exists:
   ```bash
   kubectl get clusterrolebinding metrics-reader-binding
   ```

2. Verify the ClusterRole exists:
   ```bash
   kubectl get clusterrole messaging-topology-metrics-reader
   ```

3. Ensure your token is valid and from the correct ServiceAccount

### Operator Pod Not Starting

Check the operator logs for errors:

```bash
kubectl logs -n rabbitmq-system deployment/messaging-topology-operator -c manager
```

Common issues:
- Webhook certificates not ready (wait for cert-manager to issue certificates)
- RBAC permissions missing (ensure all new ClusterRoles were created)

## Rollback

If you need to rollback to v1.18.x:

1. Delete the v1.19.0 resources:
   ```bash
   kubectl delete -f https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.19.0/messaging-topology-operator-with-certmanager.yaml
   ```

2. Reapply the v1.18.x manifests:
   ```bash
   kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.18.0/messaging-topology-operator-with-certmanager.yaml
   ```

**Note**: Your topology resources (Queues, Exchanges, etc.) are not affected by the operator upgrade and will remain intact.

## Additional Resources

- [Operator Documentation](https://www.rabbitmq.com/kubernetes/operator/operator-overview.html#topology-operator)
- [API Reference](https://github.com/rabbitmq/messaging-topology-operator/blob/main/docs/api/rabbitmq.com.ref.asciidoc)
- [Examples](https://github.com/rabbitmq/messaging-topology-operator/tree/main/docs/examples)
- [Release Notes](https://github.com/rabbitmq/messaging-topology-operator/releases/tag/v1.19.0)
