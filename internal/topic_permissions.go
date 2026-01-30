package internal

import (
	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

func GenerateTopicPermissions(p *topology.TopicPermission) rabbithole.TopicPermissions {
	return rabbithole.TopicPermissions{
		Read:     p.Spec.Permissions.Read,
		Write:    p.Spec.Permissions.Write,
		Exchange: p.Spec.Permissions.Exchange,
	}
}
