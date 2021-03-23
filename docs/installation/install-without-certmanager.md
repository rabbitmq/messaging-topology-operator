# Installation without cert-manager

Before deploying Messaging Topology Operator, you need to have:

1. A Running k8s cluster
2. RabbitMQ [Cluster Operator](https://github.com/rabbitmq/cluster-operator) installed in the k8s cluster
3. A [RabbitMQ cluster](https://github.com/rabbitmq/cluster-operator/tree/main/docs/examples) deployed using the Cluster Operator

## Installation

Download the latest release manifests https://github.com/rabbitmq/messaging-topology-operator/releases/latest/download/messaging-topology-operator.yml.

The Messaging Topology Operator has multiple [admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/). You need to generate the webhook certificate and place it in multiple places in the manifest:

1. Generate certificates for the Webhook. Certificates must be valid for `webhook-service.rabbitmq-system.svc`. `webhook-service` is the name of the webhook service object defined in release manifest `messaging-topology-operator.yml.`. `rabbitmq-system` is the namespace of the service.
2. Create a k8s secret object with name `webhook-server-cert` in namespace `rabbitmq-system`. The secret object must contain following keys: `ca.crt`, `tls.key`, and `tls.key`. For example:
    ```yaml
    apiVersion: v1
    kind: Secret
    type: kubernetes.io/tls
    metadata:
      name: webhook-server-cert
      namespace: rabbitmq-system
    data:
      ca.crt: # ca cert that can be used to validate the webhook's server certificate
      tls.crt: # generated certificate
      tls.key: # generated key
    ```
    This secret will be mounted to the operator container, where all webhooks will run from.
1. Add webhook ca certificate in downloaded release manifest `messaging-topology-operator.yml`. There are 6 admission webhooks, one for each CRD type.
Look for keyword `caBundle` in the manifest, and paste the webhook ca cert in there (6 places because there are 6 webhooks).
1. Now you are ready to deploy. If you have `kubectl` configured to access your running k8s cluster, you can then run:

```bash
kubectl apply -f messaging-topology-operator.yml
```