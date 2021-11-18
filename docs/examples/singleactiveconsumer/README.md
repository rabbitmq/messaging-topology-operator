# Single Active Consumer Example

This example leverages two CRDs in order to provide a Single Active Consumer messaging topology. In Single Active
Consumer, for any given stream queue, a number of consumer applications are configured to consume it,
however only one at a time is permitted to consume from the queue. If for any reason the application fails,
one of the standby applications will be elected to begin consuming from the stream.

Firstly, this example creates a `SuperStream` object. This behaves effectively as a regular stream queue, split into
a number of partitions. Messages published to the SuperStream's exchange will be routed to the different partitions
either by pre-determined routing keys, or dynamically split between the partitions.

Secondly, this example creates a `CompositeConsumerSet` object. This object wraps a PodSpec, which can be used to
define a Pod containing a consumer client application. The CompositeConsumerSet is defined against a SuperStream;
for each partition in the SuperStream, the CompositeConsumerSet will create a Pod to consume from that partition. By
default, the same PodSpec is used for each partition, however if you have specific partitions for different business
logics then you may define different PodSpecs per-routing key.

The CompositeConsumerSet also allows for a number of replicas to be specified. This represents the number of consumer
Pods that should be generated for each SuperStream partition. Across these replicas, only one will have the metadata
label `rabbitmq.com/active-partition-consumer` set to the name of the stream partition queue. This is the single
active consumer Pod; the PodSpec is configured to check the value of this label and only begin consuming from the
partition when this value is set. If the active consumer goes down, the label is reassigned to another replica Pod.

This example creates a SuperStream with 3 partitions, and 3 replicas of the CompositeConsumerSet, thus providing
9 consumer Pods in total. For each partition, only 1 consumer Pod is active, and 2 are on standby. The active consumers
are also spread evenly across the replicas.

Each replica is spread across a configurable topology domain: e.g. availability zones, regions. By default, this is
set to availability zones, meaning there is an even spread of active consumers across zones.
