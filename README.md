# RabbitMQ Messaging Topology Kubernetes Operator

Kubernetes operator to allow developers to create and manage [RabbitMQ](https://www.rabbitmq.com/) messaging topologies within a RabbitMQ cluster using a declarative Kubernetes API.
A Messaging topology is the collection of objects such as exchanges, queues, bindings and policies that provides specific messaging or streaming scenario. 
This operator is used with RabbitMQ clusters deployed via the [RabbitMQ Cluster Kubernetes Operator](https://github.com/rabbitmq/cluster-operator/). This repository contains [custom controllers](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) and [custom resource definitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) enabling a declarative API for RabbitMQ messaging topologies.

## Quickstart

Before deploying Messaging Topology Operator, you need to have:

1. A Running k8s cluster
2. RabbitMQ [Cluster Operator](https://github.com/rabbitmq/cluster-operator) installed in the k8s cluster
3. A [RabbitMQ cluster](https://github.com/rabbitmq/cluster-operator/tree/main/docs/examples) deployed using the Cluster Operator

If you have [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) `1.2.0` or above installed in your k8s cluster, and `kubectl` configured to access your running k8s cluster, you can then run the following command to install the Messaging Topology Operator:

```bash
kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/latest/download/messaging-topology-operator-with-certmanager.yaml
```

If you do not have cert-manager installed in your k8s cluster, you will need to generate certificates used by admission webhooks yourself and include them in the operator and webhooks manifests.
You can follow [this doc](https://www.rabbitmq.com/kubernetes/operator/install-topology-operator.html).

You can create RabbitMQ resources:

1. [Queue](./docs/examples/queues)
2. [Exchange](./docs/examples/exchanges)
3. [Binding](./docs/examples/bindings)
4. [User](./docs/examples/users)
5. [Vhost](./docs/examples/vhosts)
6. [Policy](./docs/examples/policies)
7. [Permissions](./docs/examples/permissions)
8. [Federations](./docs/examples/federations)
9. [Shovels](./docs/examples/shovels)

## Documentation

Messaging Topology Operator is covered in several guides:

 - [Operator overview](https://www.rabbitmq.com/kubernetes/operator/operator-overview.html#topology-operator)
 - [Installation](https://www.rabbitmq.com/kubernetes/operator/install-topology-operator.html)
 - [Using Messaging Topology Operator](https://www.rabbitmq.com/kubernetes/operator/using-topology-operator.html)
 - [TLS](https://www.rabbitmq.com/kubernetes/operator/tls-topology-operator.html)
 - [Troubleshooting Messaging Topology Operator](https://www.rabbitmq.com/kubernetes/operator/troubleshooting-topology-operator.html)

In addition, a number of [examples](./docs/examples), [Operator API reference](https://github.com/rabbitmq/messaging-topology-operator/blob/main/docs/api/rabbitmq.com.ref.asciidoc), and a quick [tutorial](./docs/tutorial) can be found in this repository.

The doc guides are open source. The source can be found in the [RabbitMQ website repository](https://github.com/rabbitmq/rabbitmq-website/)
under `site/kubernetes`.

## RabbitMQCluster requirements

Messaging Topology Operator is tested with the latest release of RabbitMQ [Cluster Operator](https://github.com/rabbitmq/cluster-operator).
It uses the generated default user secret from RabbitmqCluster (set in `rabbitmqcluster.status.binding`) to authenticate with RabbitMQ server.
If your RabbitmqCluster is deployed with import definitions or provided default user credentials,
the default user secret from `rabbitmqcluster.status.binding` may not be correct and Messaging Topology Operator will fail with authentication error.
If your RabbitmqCluster is configured to serve management traffic over TLS, you may need to configure the Messaging Topology Operator to trust the CA that signed the server's certificates. For more information, see [this doc](https://www.rabbitmq.com/kubernetes/operator/tls-topology-operator.html).

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/rabbitmq/messaging-topology-operator/issues), or file a new one.

Please read [contribution guidelines](CONTRIBUTING.md) if you are interested in contributing to this project.

## License

[Licensed under the MPL](LICENSE.txt), same as RabbitMQ server and cluster operator.

## Copyright

Copyright 2021 VMware, Inc. All Rights Reserved.
