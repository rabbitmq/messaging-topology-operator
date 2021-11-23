package leaderelection

import (
	corev1 "k8s.io/api/core/v1"
	"sort"
	"strconv"
)

const (
	AnnotationSuperStream = "rabbitmq.com/super-stream"
	AnnotationPartition = "rabbitmq.com/super-stream-partition"
    AnnotationConsumerIndex = "rabbitmq.com/composite-consumer-replica"
    AnnotationActiveConsumer = "rabbitmq.com/active-partition-consumer"
)

type structuredPodList map[string]map[int]corev1.Pod

func Elect(consumerPods []corev1.Pod) []corev1.Pod {
	var podList structuredPodList = make(map[string]map[int]corev1.Pod)
	for _, pod := range consumerPods {
		if _, ok := podList[pod.ObjectMeta.Labels[AnnotationPartition]]; !ok {
			podList[pod.ObjectMeta.Labels[AnnotationPartition]] = make(map[int]corev1.Pod)
		}
		replicaPods := podList[pod.ObjectMeta.Labels[AnnotationPartition]]
		replicaIndex, _ := strconv.Atoi(pod.ObjectMeta.Labels[AnnotationConsumerIndex])
		replicaPods[replicaIndex] = pod
	}

	var sortedPartitionKeys []string
	for key := range podList {
		sortedPartitionKeys = append(sortedPartitionKeys, key)
	}
	sort.Strings(sortedPartitionKeys)

	for partitionIndex, partition := range sortedPartitionKeys {
		consumerIndex := assignPartitionToReplica(partitionIndex, len(podList[partition]))

		var sortedReplicaKeys []int
		for key := range podList[partition] {
			sortedReplicaKeys = append(sortedReplicaKeys, key)
		}
		sort.Ints(sortedReplicaKeys)
		for keyIndex, replicaIndex := range sortedReplicaKeys {
			pod := podList[partition][replicaIndex]
			if keyIndex == consumerIndex {
				pod.ObjectMeta.Labels[AnnotationActiveConsumer] = pod.ObjectMeta.Labels[AnnotationPartition]
			} else {
				pod.ObjectMeta.Labels[AnnotationActiveConsumer] = ""
			}
		}
	}
	return consumerPods
}

func assignPartitionToReplica(partitionIndex, consumerReplicaCount int) int {
	return partitionIndex % consumerReplicaCount
}