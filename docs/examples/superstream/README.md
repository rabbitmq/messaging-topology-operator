# SuperStream example

This example creates a `SuperStream` object. This behaves effectively as a regular stream queue, split into
a number of partitions. Messages published to the SuperStream will be routed to the different partitions
either by pre-determined routing keys, or dynamically split between the partitions.
