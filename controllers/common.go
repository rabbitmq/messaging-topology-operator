package controllers

// common error messages shared across controllers
const (
	failedStatusUpdate         = "failed to update object status"
	failedMarshalSpec          = "failed to marshal spec"
	failedGenerateRabbitClient = "failed to generate http rabbitClient"
	failedParseClusterRef      = "failed to retrieve cluster from reference"
	noSuchRabbitDeletion       = "RabbitmqCluster is already gone: cannot find its connection secret"
)

// names for each of the controllers
const (
	VhostControllerName             = "vhost-controller"
	QueueControllerName             = "queue-controller"
	ExchangeControllerName          = "exchange-controller"
	BindingControllerName           = "binding-controller"
	UserControllerName              = "user-controller"
	PolicyControllerName            = "policy-controller"
	PermissionControllerName        = "permission-controller"
	SchemaReplicationControllerName = "schema-replication-controller"
)
