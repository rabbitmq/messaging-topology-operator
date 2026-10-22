# User examples

This section contains the examples for creating RabbitMQ users.

Messaging Topology Operator creates users with generated credentials by default. To create RabbitMQ users with provided credentials, you can reference a kubernetes secret object (`spec.importCredentialsSecret`) with the following keys in its Data field:

* `username` – Must be present or the import will fail.
* `passwordHash` – The SHA-512 hash of the password, as described in [RabbitMQ Docs](https://www.rabbitmq.com/docs/passwords). If the hash is an empty string, a passwordless user will be created.
* `password` – Plain-text password. Will be used only if the `passwordHash` key is missing.  

See [userPreDefinedCreds.yaml](./userPreDefinedCreds.yaml), [userWithPasswordHash.yaml](userWithPasswordHash.yaml), [passwordlessUser.yaml](passwordlessUser.yaml) and [publish-consume-user.yaml](./publish-consume-user.yaml) as examples.

To use the User's `metadata.name` as the RabbitMQ username and let the Operator generate a random password, set `spec.usernameFromMeta: true`. The username and password are stored in the Operator-owned Secret `<name>-user-credentials`. `usernameFromMeta` cannot be set together with `importCredentialsSecret`.
See [setUsernamewithGenPass.yaml](./setUsernamewithGenPass.yaml) as an example.

Between [Messaging Topology Operator v1.10.0](https://github.com/rabbitmq/messaging-topology-operator/releases/tag/v1.10.1) and v1.19, an `importCredentialsSecret` containing only `username` made the Operator generate the password. This is no longer supported: such a User now fails with `neither passwordHash nor password is set in credentials`. For a new User, use `spec.usernameFromMeta: true` to use `metadata.name` as the username with a generated password. To migrate an existing User created before v1.20 that used a username-only Secret while preserving its credentials:

1. Before editing the User, label its existing generated Secret so the Operator's filtered Secret cache can see it. Replace `<name>` and `<namespace>` with the User's name and namespace:

   ```sh
   kubectl label secret <name>-user-credentials \
     -n <namespace> rabbitmq.com/topology-operator=true --overwrite
   ```

   Without this label, the Operator cannot find the existing Secret and attempts to create it, failing with `AlreadyExists`.
2. Remove `spec.importCredentialsSecret` from the User and leave `spec.usernameFromMeta` unset (or `false`). The Operator reuses the username and password in the existing `<name>-user-credentials` Secret, even when that username differs from `metadata.name`. Do not delete this generated Secret: recreating it changes the credentials and can break existing clients.
3. Verify that the User is Ready and `status.credentials.name` points to `<name>-user-credentials`. The now unused username-only import Secret can then be deleted.

You may set `spec.usernameFromMeta: true` during migration only if the existing generated Secret's `username` already equals the User's `metadata.name`. Otherwise, the User reports a mismatch error; leave `usernameFromMeta` unset to preserve its credentials. Changing to a different username creates a different RabbitMQ user and requires migrating clients and permissions; the Operator does not delete the previous RabbitMQ user.

If no generated Secret exists (for example, the User was first created on v1.20 or later), removing `importCredentialsSecret` generates new credentials. To retain a specific username in that case, provide a complete import Secret containing that username and a password or password hash.

Any secret referenced by `importCredentialsSecret` must carry the label `rabbitmq.com/topology-operator: "true"`. The Operator's Secret cache is restricted to labeled secrets, and the admission webhook will reject a User that references an unlabeled or missing secret.

The Messaging Topology Operator watches the `importCredentialsSecret` and automatically rotates the RabbitMQ user's password when the secret's `password` or `passwordHash` changes — no manual reconcile trigger is required. No separate `-user-credentials` secret is generated for users created this way; the `importCredentialsSecret` itself is the source of truth.
