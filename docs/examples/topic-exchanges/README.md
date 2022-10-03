# Topic Exchange Example

In this example, we will create two topic exchanges, and a user that have read and write access to them in the default `/` vhost.

Before creating any topology objects with Messaging Topology Operator, please deploy a RabbitmqCluster named `test`, or any name you see fit. You will need to modify the example manifests if the RabbitmqCluster name is something other than `test`.

After the RabbitMQ cluster is successfully created, you can first create the two topic exchanges and a user by:

```bash
kubectl apply -f topic-exchanges.yaml # create topic exchanges `topic-a` and `topic-b` in default vhost `/`
kubectl apply -f user.yaml # create user `example`
```

Then you can grant user `example` permissions to both topic exchanges by:

```bash
kubectl apply -f user-topic-permissions.yaml
```

This will create two `topicpermissions.rabbitmq.com` objects: one for managing user `example` permissions to `topic-a` and the other one manages user `example` permissions to `topic-b`.

To clean up, you can run:

```bash
kubectl delete -f user-topic-permissions.yaml -f user.yaml -f topic-exchanges.yaml
```

