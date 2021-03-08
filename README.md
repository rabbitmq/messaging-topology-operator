# RabbitMQ Messaging Topology Kubernetes Operator

Kubernetes operator to manage [RabbitMQ](https://www.rabbitmq.com/) messaging topologies within a RabbitMQ cluster deployed via the [RabbitMQ Cluster Kubernetes Operator](https://github.com/rabbitmq/cluster-operator/). This repository contains [custom controllers](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) and [custom resource definitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) enabling a declarative API for RabbitMQ messaging topologies.

## Quickstart

Before deploying Messaging Topology Operator, you need to have:

1. A Running k8s cluster
2. RabbitMQ [Cluster Operator](https://github.com/rabbitmq/cluster-operator) installed in the k8s cluster
3. A [RabbitMQ cluster](https://github.com/rabbitmq/cluster-operator/tree/main/docs/examples) deployed using the Cluster Operator

If you have `kubectl` configured to access your running k8s cluster, you can then run the following command to install the Messaging Topology Operator:

```bash
kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/download/latest/messaging-topology-operator.yml
```

You can create RabbitMQ resources:

1. [Queue](./docs/examples/queues)
2. [Exchange](.docs/examples/exchanges)
3. [Binding](.docs/examples/bindings)
4. [User](.docs/examples/users)
5. [Vhost](.docs/examples/vhosts)

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/rabbitmq/messaging-topology-operator/issues), or file a new one.

Please read [contribution guidelines](CONTRIBUTING.md) if you are interested in contributing to this project.

## License

[Licensed under the MPL](LICENSE.txt), same as RabbitMQ server and cluster operator.

## Copyright

Copyright 2021 VMware, Inc. All Rights Reserved.
