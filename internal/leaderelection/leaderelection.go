package leaderelection

import (
	corev1 "k8s.io/api/core/v1"
	"sort"
	"strconv"
)

const (
	AnnotationPartition = "rabbitmq.com/super-stream-partition"
    AnnotationConsumerIndex = "rabbitmq.com/composite-consumer-replica"
    AnnotationActiveConsumer = "rabbitmq.com/active-partition-consumer"
)

type structuredPodList map[string][]*corev1.Pod

func Elect(consumerPods []*corev1.Pod) {
	var numberOfReplicas = 0
	for _, pod := range consumerPods {
		index, _ := strconv.Atoi(pod.ObjectMeta.Labels[AnnotationConsumerIndex])
		if index+1 > numberOfReplicas {numberOfReplicas = index+1}
	}

	var podList structuredPodList = make(map[string][]*corev1.Pod)
	for _, pod := range consumerPods {
		if _, ok := podList[pod.ObjectMeta.Labels[AnnotationPartition]]; !ok {
			podList[pod.ObjectMeta.Labels[AnnotationPartition]] = make([]*corev1.Pod, numberOfReplicas)
		}
		replicaPods := podList[pod.ObjectMeta.Labels[AnnotationPartition]]
		replicaIndex, _ := strconv.Atoi(pod.ObjectMeta.Labels[AnnotationConsumerIndex])
		replicaPods[replicaIndex] = pod
	}

	sortedPartitionKeys := make([]string, 0, len(podList)/numberOfReplicas)
	for key := range podList {
		sortedPartitionKeys = append(sortedPartitionKeys, key)
	}
	sort.Strings(sortedPartitionKeys)

	for partitionIndex, partition := range sortedPartitionKeys {
		consumerIndex := assignPartitionToReplica(partitionIndex, numberOfReplicas)
		for podIndex, pod := range podList[partition] {
			if podIndex == consumerIndex {
				pod.ObjectMeta.Labels[AnnotationActiveConsumer] = pod.ObjectMeta.Labels[AnnotationPartition]
			} else {
				pod.ObjectMeta.Labels[AnnotationActiveConsumer] = ""
			}
		}
	}
}

func assignPartitionToReplica(partitionIndex, consumerReplicaCount int) int {
	return partitionIndex % consumerReplicaCount
}