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
necessary for the operator container to have the `VAULT_ADDR` environment
variable set to the URL of the Vault server API.

The following environment variables may be optionally set if the defaults are
not applicable.

- `OPERATOR_VAULT_ROLE` the name of the Vault role that is used when accessing credentials. Defaults to "messaging-topology-operator"
- `OPERATOR_VAULT_NAMESPACE` the [Vault namespace](https://www.vaultproject.io/docs/enterprise/namespaces) to use when the Messaging Topology operator is authenticating. If not set then the default Vault namespace is assumed
- `OPERATOR_VAULT_AUTH_PATH` the auth path that the operator ought to use when authenticating to Vault. Default behaviour is to use the “auth/kubernetes” path

In this example the Vault configuration is carried out automatically using
the  [setup.sh](./setup.sh) script.

## Usage

This example requires:
1. RabbitMQ Cluster operator is installed
2. RabbitMQ Messaging Topology operator is installed

Run the [setup.sh](./setup.sh) script to install Vault to the current
namespace of the Kubernetes cluster. It also carries out the following
activities in preparation for running the test script.

- creates the default user credentials for cluster `cluster-vault-a`
- creates a Vault policy called `messaging-topology-operator-policy` that grants _read access_ to the credentials of `cluster-vault-a`.
- creates a separate Vault policy called `cluster-vault-a-policy` that grants _read write access_ to the credentials of `cluster-vault-a`.
- creates the Vault role `messaging-topology-operator` which references the Vault policy `messaging-topology-operator-policy` to permit the service account for the Messaging Topology operator to read the cluster credentials
- creates the Vault role `rabbitmq-cluster` which references the policy `cluster-vault-a-policy` to permit the service account for the RabbitMQ cluster `cluster-vault-a` to read and write the cluster credentials

Note that an alternative to the above approach of configuring each RabbitMQ
cluster with its own Vault policy and role would be to make use of
[wildcards](https://www.vaultproject.io/docs/concepts/policies#policy-syntax)
in the Vault policy path value. For instance, a Vault policy
which grants access to the path `secret/data/rabbitmq/some-group-name/*`
could be associated with a single Vault role used by a group
of RabbitMQ clusters. The role's `bound_service_account_names` value would
need to include the name of each group member's Kubernetes service account.
Similarly, the role's `bound_service_account_namespaces` would need to
include the name of each member's Kubernetes namespace.

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

