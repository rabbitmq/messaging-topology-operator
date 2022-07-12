# User examples

This section contains 3 examples for creating RabbitMQ users.
Messaging Topology Operator creates users with generated credentials by default. To create RabbitMQ users with provided credentials, you can reference a kubernetes secret object contains keys `username` and `password` in its Data field.
See [userPreDefinedCreds.yaml](./userPreDefinedCreds.yaml) and [publish-consume-user.yaml](./publish-consume-user.yaml) as examples.
Note that Messaging Topology Operator does not watch the provided secret and updating the secret object won't update actual user credentials.
If you wish to update user credentials, you can update the secret and then add a label or annotation to the User object to trigger a Reconile loop.
