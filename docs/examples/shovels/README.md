# Shovel Example

In this example, we will declare a dynamic Shovel that moves messages from a clasic queue to a quorum queue in the same RabbitmqCluster.

Before creating any topology objects with Messaging Topology Operator, please deploy a RabbitmqCluster named `example-rabbit`, or any name you see fit.
You can pick any RabbitmqCluster example from the [cluster operator repo](https://github.com/rabbitmq/cluster-operator/blob/main/docs/examples).

After the RabbitMQ cluster is successfully created, you need to get username and password of the default user for this RabbitMQ cluster:

```bash
kubectl get secret example-rabbit-default-user -o jsonpath='{.data.username}' | base64 --decode
kubectl get secret example-rabbit-user -o jsonpath='{.data.password}' | base64 --decode
```
Save the username and password, because we need both later to construct the source and destination URI.

This example includes (please create in order):

1. two vhosts: 'source' and 'destination'
1. a Kubernetes secret 'shovel-secret' containing the shovel source and destination URIs
1. a classic queue 'source-queue' in vhost 'source' vhost
1. a quorum queue 'destination-queue' in vhost 'destination'
1. shovel 'shovel-example' between queue 'source-queue' and queue 'destination-queue'

After all topology objects are created, messages in 'source queue'
will be moved into 'destination-queue'.

Learn [more about RabbitMQ Dynamic Shovel](https://www.rabbitmq.com/shovel-dynamic.html).
