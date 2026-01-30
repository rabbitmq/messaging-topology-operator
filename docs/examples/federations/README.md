# Federation Example

In this example, we will create federation between exchanges from two different vhosts in the same RabbitmqCluster.

Before creating any topology objects with Messaging Topology Operator, please deploy a RabbitmqCluster named `example-rabbit`, or any name you see fit.
You will need to enable the `rabbitmq_federation` plugin and here is an example from the [cluster operator repo](https://github.com/rabbitmq/cluster-operator/blob/main/docs/examples/plugins/rabbitmq.yaml) about enabling additional plugins.

After the RabbitMQ cluster is successfully created, you need to get username and password of the default user for this RabbitMQ cluster:

```bash
kubectl get secret example-rabbit-default-user -o jsonpath='{.data.username}' | base64 --decode
kubectl get secret example-rabbit-user -o jsonpath='{.data.password}' | base64 --decode
```
Save the username and password, because we need both later to construct federation upstream URI.

This example includes (please create in order):

1. two vhosts: 'upstream' and 'downstream'
1. a Kubernetes secret 'federation-uri' containing the federation upstream URI
1. queue 'upstream-queue' in 'upstream' vhost
1. queue 'downstream-queue' in 'downstream-vhost'
1. fanout exchanges in both 'upstream' and 'downstream' vhost
1. bindings between fanout exchanges to 'upstream-queue' in 'upstream' vhost and 'downstream-queue' in 'downstream-vhost'
1. a federation upstream named 'origin' in 'downstream' vhost
1. a policy between federation upstream 'origin' and the fanout exchange in 'downstream' vhost

After all topology objects are created, messages published to the fanout exchange in "upstream" will be published to the
'upstream-queue' in 'upstream-vhost' and federated to the 'downstream-queue' in 'downstream' vhost.

Learn [more about RabbitMQ Federation](https://www.rabbitmq.com/federation.html).
