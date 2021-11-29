# Single Active Consumer Example

This example leverages two CRDs in order to provide a Single Active Consumer messaging topology. In Single Active
Consumer, for any given stream queue, a number of consumer applications are configured to consume it,
however only one at a time is permitted to consume from the queue. If for any reason the application fails,
a new Pod is spun up to continue consuming from the stream.

Firstly, this example creates a `SuperStream` object. This behaves effectively as a regular stream queue, split into
a number of partitions. Messages published to the SuperStream's exchange will be routed to the different partitions
either by pre-determined routing keys, or dynamically split between the partitions.

Secondly, this example creates a `CompositeConsumer` object. This object wraps a PodSpec, which can be used to
define a Pod containing a consumer client application. The CompositeConsumer is defined against a SuperStream;
for each partition in the SuperStream, the CompositeConsumer will create a Pod to consume from that partition. By
default, the same PodSpec is used for each partition, however if you have specific partitions for different business
logics then you may define different PodSpecs per-routing key.
