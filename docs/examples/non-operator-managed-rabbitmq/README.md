## Supporting to any RabbitMQ clusters

This is an example on how to create RabbitMQ topology objects in RabbitMQ clusters that's not Cluster Operator managed.
The example first creates a Kubernetes secret object that contains keys 'username', 'password' and 'uri'.
When creating a `queue.rabbitmq.com` resource, the secret is provided instead of a name reference
to a RabbitmqCluster.

