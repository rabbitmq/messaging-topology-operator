# User examples

This section contains the examples for creating RabbitMQ users.

Messaging Topology Operator creates users with generated credentials by default. To create RabbitMQ users with provided credentials, you can reference a kubernetes secret object (`spec.importCredentialsSecret`) with the following keys in its Data field:

* `username` – Must be present or the import will fail.
* `passwordHash` – The SHA-512 hash of the password, as described in [RabbitMQ Docs](https://www.rabbitmq.com/docs/passwords). If the hash is an empty string, a passwordless user will be created.
* `password` – Plain-text password. Will be used only if the `passwordHash` key is missing.  

See [userPreDefinedCreds.yaml](./userPreDefinedCreds.yaml), [userWithPasswordHash.yaml](userWithPasswordHash.yaml), [passwordlessUser.yaml](passwordlessUser.yaml) and [publish-consume-user.yaml](./publish-consume-user.yaml) as examples.

To use the User's `metadata.name` as the RabbitMQ username and let the Operator generate a random password, set `spec.usernameFromMeta: true`. The username and password are stored in the Operator-owned Secret `<name>-user-credentials`. `usernameFromMeta` cannot be set together with `importCredentialsSecret`.
See [setUsernamewithGenPass.yaml](./setUsernamewithGenPass.yaml) as an example.

Between [Messaging Topology Operator v1.10.0](https://github.com/rabbitmq/messaging-topology-operator/releases/tag/v1.10.1) and v1.19, an `importCredentialsSecret` containing only `username` made the Operator generate the password. This is no longer supported: such a User now fails with `neither passwordHash nor password is set in credentials`. `spec.usernameFromMeta` replaces it. To migrate a User that used a username-only secret:

1. Edit the User: remove `spec.importCredentialsSecret` and set `spec.usernameFromMeta: true`.
2. If the User was created before v1.20, its `<name>-user-credentials` Secret still holds the generated password and the existing password is kept. This requires that Secret's `username` to equal the User's `metadata.name`; otherwise the User reports an error and you must pick a User name matching the existing username (or delete the Secret and accept a new generated password; note that the Operator does not remove the RabbitMQ user with the old username, so delete it manually if it is no longer needed).
3. The now unused username-only Secret can be deleted.

Any secret referenced by `importCredentialsSecret` must carry the label `rabbitmq.com/topology-operator: "true"`. The Operator's Secret cache is restricted to labeled secrets, and the admission webhook will reject a User that references an unlabeled or missing secret.

The Messaging Topology Operator watches the `importCredentialsSecret` and automatically rotates the RabbitMQ user's password when the secret's `password` or `passwordHash` changes — no manual reconcile trigger is required. No separate `-user-credentials` secret is generated for users created this way; the `importCredentialsSecret` itself is the source of truth.
