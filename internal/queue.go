package internal

import (
	"encoding/json"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// GenerateQueueSettings generates rabbithole.QueueSettings for the given Queue
// queue.Spec.Arguments (type k8s apimachinery runtime.RawExtensions) is unmarshalled
// Unmarshall stores: bool, for JSON booleans
// float64, for JSON numbers
// string, for JSON strings
// []interface{}, for JSON arrays
// map[string]interface{}, for JSON objects
// nil for JSON null
// See: https://golang.org/pkg/encoding/json/#Unmarshal
func GenerateQueueSettings(q *topologyv1beta1.Queue) (*rabbithole.QueueSettings, error) {
	arguments := make(map[string]interface{})
	if q.Spec.Arguments != nil {
		if err := json.Unmarshal(q.Spec.Arguments.Raw, &arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshall queue arguments: %v", err)
		}
	}

	// bug in rabbithole.QueueSettings; setting queue type in QueueSettings.Type not working
	// needs to set queue type in QueueSettings.Arguments
	if q.Spec.Type != "" {
		arguments["x-queue-type"] = q.Spec.Type
	}

	return &rabbithole.QueueSettings{
		Durable:    q.Spec.Durable,
		AutoDelete: q.Spec.AutoDelete,
		Arguments:  arguments,
	}, nil
}
