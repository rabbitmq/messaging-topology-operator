# HashiCorp Vault Support Example

The RabbitMQ Cluster Operator supports storing RabbitMQ default user (admin)
credentials and RabbitMQ server certificates in
[HashiCorp Vault](https://www.vaultproject.io/) and in K8 Secrets. And
accordingly, the RabbitMQ Messaging Topology operator can operate on clusters
which are vault-enabled or k8s-secrets enabled.

As explained in [this KubeCon talk](https://youtu.be/w0k7MI6sCJg?t=177) there
are four different approaches in Kubernetes to consume external secrets:

1. Direct API
2. Controller to mirrors secrets in K8s
3. Sidecar + MutatingWebhookConfiguration
4. Secrets Store CSI Driver

The RabbitMQ Cluster Operator integrates with Vault using the third approach
(Sidecar + MutatingWebhookConfiguration) whereas the RabbitMQ Messaging
Topology Operator uses the first approach (Direct API).

## Vault-related configuration required

The Vault server must have the version 2 key value secret engine and the 
[Vault Kubernetes auth method](https://www.vaultproject.io/docs/auth/kubernetes)
enabled.

```bash
$ vault secrets enable -path=secret kv-v2
```

```bash
$ vault auth enable kubernetes
```

In order for the RabbitMQ Messaging Topology operator to authenticate with
a Vault server and access RabbitMQ cluster default user credentials it is
necessary for the operator container to have one or more environment variables
set. 

- `VAULT_ADDR` should be set to the URL of the Vault server API
- `OPERATOR_VAULT_ROLE` should be set to the name of the Vault role that is used when accessing credentials. This defaults to "messaging-topology-operator"
- `OPERATOR_VAULT_NAMESPACE` may be set to the Vault namespace to use when the Messaging Topology operator is authenticating. If not set then the default Vault namespace is assumed
- `OPERATOR_VAULT_AUTH_PATH` may be set to the auth path that the operator ought to use when authenticating to Vault. If not set then it is assumed that the “auth/kubernetes” path should be used

In this example the Vault configuration is carried out automatically using
the  [setup.sh](./setup.sh) script.

## Usage

This example requires:
1. RabbitMQ Cluster operator is installed
2. RabbitMQ Messaging Topology operator is installed

Run the [setup.sh](./setup.sh) script to install Vault to the current
Kubernetes cluster. This will also create the necessary credentials, 
policy, and configuration in preparation for running the example.

Next, run the [test.sh](./test.sh) script. This automates the execution of the
following steps:
1. Update the existing Messaging Topology operator deployment to set the `VAULT_ADDR` and `OPERATOR_VAULT_ROLE` environment variables. This triggers a redeployment. 
2. Create two RabbitMQ clusters: `cluster-vault-a` which uses Vault to store its admin credentials and `cluster-b` which uses the Kubernetes secret alternative.
3. Creates a queue in each of the RabbitMQ clusters. This demonstrates that the Messaging Topology operator is able to use both access methods to interact with the clusters.
4. Deletes both queues from the RabbitMQ clusters.
5. Updates the Messaging Topology operator configuration to set an incorrect value of `OPERATOR_VAULT_ROLE`. Doing this intentionally breaks the operator authentication with Vault. This triggers a redeployment.
6. Repeats the creation of a queue in each of the RabbitMQ clusters. This will succeed for the `cluster-b` but not for `cluster-vault-a` which will no longer be able to connect to Vault. 
7. Updates the Messaging Topology operator configuration a further time to restore the correct Vault configuration. This triggers a redeployment.
8. Deletes both queues.
9. Deletes both RabbitMQ clusters.
10. Removes the `VAULT_ADDR` and `OPERATOR_VAULT_ROLE` environment variables from the Messaging Topology operator.
11. Removes Vault.

