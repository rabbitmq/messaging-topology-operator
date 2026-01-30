# Shovel Example

Shovel can move messages between move messages reliably and continually (typically between queues).
In this example, we will declare a dynamic Shovel that moves messages from a classic queue to a quorum queue in the same RabbitmqCluster.
The example can be used when you want to migrate from classic or mirrored queues to quorum queues.

Before creating any topology objects with Messaging Topology Operator, please deploy a RabbitmqCluster named `example-rabbit`, or any name you see fit. You will need to enable the `rabbitmq_shovel` plugin and here is an example from the [cluster operator repo](https://github.com/rabbitmq/cluster-operator/blob/main/docs/examples/plugins/rabbitmq.yaml) about enabling additional plugins.

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

## Supported Protocols
Shovel supports AMQP 0.9.1 and AMQP 1.0 protocol. Set `spec.srcProtocol` or `spec.destProtocol` to configure protocol that the plugin uses to connect to source and destination.
Note that if protocol information is omitted Shovel plugin will default to AMQP 0.9.1.
To use AMQP 1.0 protocol you have to set either source or destination protocol, or both, e.g.

```yaml
apiVersion: rabbitmq.com/v1beta1
kind: Shovel
spec:
...
  srcProtocol: amqp10
  srcAddress: "/some-exchange" # required if srcProtocol is amqp10
  destProtocol: amqp10
  destAddress: "/some-exchange" # required if destProtocol is amqp10
```

Some configuration are protocol specific, e.g.:

```yaml
apiVersion: rabbitmq.com/v1beta1
kind: Shovel
spec:
...
  srcQueue: # amqp091
  srcConsumerArgs: # amqp091
  destQueue: # amqp091
  srcExchange: # amqp091
  srcExchangeKey: # amqp091
  srcAddress: # amqp10; required if using AMQP 1.0 to connect to source

  destQueue: # amqp091
  destExchange: # amqp091
  destExchangeKey: # amqp091
  destPublishProperties: # amqp091
  destAddress: # amqp10; required if using AMQP 1.0 to connect to destination
  destApplicationProperties: # amqp10
  destProperties: # amqp10
  destMessageAnnotations: # amqp10
```

Learn [more about RabbitMQ Dynamic Shovel](https://www.rabbitmq.com/shovel-dynanamic.html).
