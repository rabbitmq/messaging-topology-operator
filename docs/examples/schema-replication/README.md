# Schema Replication Example

VMware Tanzu RabbitMQ supports continuous schema definition replication to a remote cluster, which makes it easy to run a hot standby cluster for disaster recovery.

This feature is not available in the open source RabbitMQ distribution.

Schema Replication can be configured using the topology operator.

In the upstream-rabbitmq.yaml and downstream-rabbitmq.yaml files are present the configuration definitions in order to setup two RabbitMQ clusters: an upstream cluster and a downstream cluster with the schema replication plugin enabled and relative prerequisites (user, vhost, permissions).

After this we can use schema-replication-upstream.yaml and schema-replication-downstream.yaml to setup the schema-replication settings with the topology operator.

Namespaces upstream and downstream needs to be created in advance to run the tests.

Note also that as this feature is just supported by the VMware Tanzu version you need to follow up this guideline https://docs.vmware.com/en/VMware-Tanzu-RabbitMQ-for-Kubernetes/1.2/tanzu-rmq/GUID-installation.html to install operators and deploy RabbitMQ clusters and in particular to configure the secrets to pull the commercial images from the VMware Tanzu repository (Harbor)  

Learn [more about RabbitMQ Schema Definition Replication](https://www.rabbitmq.com/definitions-standby.html).
