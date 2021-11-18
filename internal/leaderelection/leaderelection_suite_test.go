package leaderelection_test

import (
	"fmt"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/rabbitmq/messaging-topology-operator/internal/leaderelection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLeaderelection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Leaderelection Suite")
}

func generateTestPodSet(numberOfPartitions, numberOfConsumerSets int) []*corev1.Pod {
	var pods []*corev1.Pod
	for i := 0; i < numberOfPartitions; i++ {
		for j := 0; j < numberOfConsumerSets; j++ {
			pods = append(pods, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("consumer-partition.%d-replica.%d", i, j),
					Labels: map[string]string{
						leaderelection.AnnotationActiveConsumer: "",
						leaderelection.AnnotationConsumerIndex:  strconv.Itoa(j),
						leaderelection.AnnotationPartition:      fmt.Sprintf("partition.%d", i),
					},
				},
			},
			)
		}
	}
	return pods
}

func BeBalanced() types.GomegaMatcher {
	partitions := make(map[string]consumers)
	replicas := make(map[string]consumers)
	return &BeBalancedMatcher{
		partitions: partitions,
		replicas: replicas,
	}
}

type BeBalancedMatcher struct {
	partitions map[string]consumers
	replicas map[string]consumers
	additionalFailureContext string
}

type consumers struct {
	activeConsumers []string
	standbyConsumers []string
}

func newEmptyConsumerList() consumers {
	return consumers{
		activeConsumers:  []string{},
		standbyConsumers: []string{},
	}
}

func (matcher *BeBalancedMatcher) Match(actual interface{}) (bool, error) {
	pods, ok := actual.([]*corev1.Pod)
	if !ok {
		return false, fmt.Errorf("BeBalanced must be passed a list of Pod pointers. Got\n%s", format.Object(actual, 1))
	}

	for _, pod := range pods {
		podPartition := pod.ObjectMeta.Labels[leaderelection.AnnotationPartition]
		podConsumerIndex := pod.ObjectMeta.Labels[leaderelection.AnnotationConsumerIndex]

		partitionConsumers, ok := matcher.partitions[podPartition]
		if !ok {
			matcher.partitions[podPartition] = newEmptyConsumerList()
		}
		replicaConsumers, ok := matcher.replicas[podConsumerIndex]
		if !ok {
			matcher.replicas[podConsumerIndex] = newEmptyConsumerList()
		}

		if pod.ObjectMeta.Labels[leaderelection.AnnotationActiveConsumer] != "" {
			partitionConsumers.activeConsumers = append(partitionConsumers.activeConsumers, pod.Name)
			replicaConsumers.activeConsumers = append(replicaConsumers.activeConsumers, pod.Name)
		} else {
			partitionConsumers.standbyConsumers = append(partitionConsumers.standbyConsumers, pod.Name)
			replicaConsumers.standbyConsumers = append(replicaConsumers.standbyConsumers, pod.Name)
		}
		matcher.partitions[podPartition] = partitionConsumers
		matcher.replicas[podConsumerIndex] = replicaConsumers
	}

	// Each partition should only have one active consumer on it
	for partition, consumers := range matcher.partitions {
		if len(consumers.activeConsumers) != 1 {
			matcher.additionalFailureContext = fmt.Sprintf(
				"Expected partition %s to have exactly one active consumer, but had %d instead.",
				partition,
				len(consumers.activeConsumers),
			)
			return false, nil
		}
	}

	// The difference between the max/min number of active consumers on each replica should be at most 1
	var activeConsumerCounts []int
	for _, consumers := range matcher.replicas {
		activeConsumerCounts = append(activeConsumerCounts, len(consumers.activeConsumers))
	}
	sort.Ints(activeConsumerCounts)
	max := activeConsumerCounts[len(activeConsumerCounts)-1]
	min := activeConsumerCounts[0]
	if max - min > 1 {
		matcher.additionalFailureContext = fmt.Sprintf(
			"Expected the difference in number of active consumers across replicas to be at most 1; instead replicas had between %d and %d active consumers",
			min,
			max,
		)
		return false, nil
	}
	return true, nil
}

func (matcher *BeBalancedMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Pod consumer distribution was not balanced: %s\n%s", matcher.additionalFailureContext, matcher.DistributionErrorString())
}

func (matcher *BeBalancedMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Pod consumer distribution was not expected to be balanced.\n%s", matcher.DistributionErrorString())
}

func (matcher *BeBalancedMatcher) DistributionErrorString() string {
	var errString = "Distribution across partitions:"
	for partition, consumers := range matcher.partitions {
		errString += fmt.Sprintf("\n%s:", partition)
		errString += "\n  Active consumers:\n"
		for _, consumer := range consumers.activeConsumers {
			errString += fmt.Sprintf("    - %s\n", consumer)
		}
		errString += "\n  Standby consumers:\n"
		for _, consumer := range consumers.standbyConsumers {
			errString += fmt.Sprintf("    - %s\n", consumer)
		}
	}

	errString += "Distribution across replicas:"
	for replica, consumers := range matcher.replicas {
		errString += fmt.Sprintf("\n%s:", replica)
		errString += "\n  Active consumers:\n"
		for _, consumer := range consumers.activeConsumers {
			errString += fmt.Sprintf("    - %s\n", consumer)
		}
		errString += "\n  Standby consumers:\n"
		for _, consumer := range consumers.standbyConsumers {
			errString += fmt.Sprintf("    - %s\n", consumer)
		}
	}
	return errString
}
