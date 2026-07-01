# User examples

This section contains the examples for creating RabbitMQ users.

Messaging Topology Operator creates users with generated credentials by default. To create RabbitMQ users with provided credentials, you can reference a kubernetes secret object (`spec.importCredentialsSecret`) with the following keys in its Data field:

* `username` – Must be present or the import will fail.
* `passwordHash` – The SHA-512 hash of the password, as described in [RabbitMQ Docs](https://www.rabbitmq.com/docs/passwords). If the hash is an empty string, a passwordless user will be created.
* `password` – Plain-text password. Will be used only if the `passwordHash` key is missing.  

See [userPreDefinedCreds.yaml](./userPreDefinedCreds.yaml), [userWithPasswordHash.yaml](userWithPasswordHash.yaml), [passwordlessUser.yaml](passwordlessUser.yaml) and [publish-consume-user.yaml](./publish-consume-user.yaml) as examples.

From [Messaging Topology Operator v1.10.0](https://github.com/rabbitmq/messaging-topology-operator/releases/tag/v1.10.1), you can provide a username and reply on the Operator to generate its password for you.
See [setUsernamewithGenPass.yaml](./setUsernamewithGenPass.yaml) as an example.

Any secret referenced by `importCredentialsSecret` must carry the label `rabbitmq.com/topology-operator: "true"`. The Operator's Secret cache is restricted to labeled secrets, and the admission webhook will reject a User that references an unlabeled or missing secret.

The Messaging Topology Operator watches the `importCredentialsSecret` and automatically rotates the RabbitMQ user's password when the secret's `password` or `passwordHash` changes — no manual reconcile trigger is required. No separate `-user-credentials` secret is generated for users created this way; the `importCredentialsSecret` itself is the source of truth.
