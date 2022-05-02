# Schema Replication Example

VMware Tanzu RabbitMQ supports continuous schema definition replication to a remote cluster, which makes it easy to run a hot standby cluster for disaster recovery.

This feature is not available in the open source RabbitMQ distribution.

Schema Replication can be configured using the topology operator.

In the upstream-rabbitmq.yaml and downstream-rabbitmq.yaml files are present the configuration definitions in order to setup two RabbitMQ clusters with the schema replication plugin enabled and relative prerequisites (user, vhost, permissions).

After this we can use schema-replication-upstream.yaml and schema-replication-downstream.yaml to setup the schema-replication settings with the topology operator.

Learn [more about RabbitMQ Schema Definition Replication](https://www.rabbitmq.com/definitions-standby.html).
