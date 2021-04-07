package controllers

// common error messages shared across controllers
const (
	failedStatusUpdate         = "Failed to update object status"
	failedMarshalSpec          = "Failed to marshal spec"
	failedGenerateRabbitClient = "Failed to generate http rabbitClient"
	noSuchRabbitDeletion       = "RabbitmqCluster is already gone: cannot find its connection secret"
)

// names for each of the controllers
const (
	VhostControllerName      = "vhost-controller"
	QueueControllerName      = "queue-controller"
	ExchangeControllerName   = "exchange-controller"
	BindingControllerName    = "binding-controller"
	UserControllerName       = "user-controller"
	PolicyControllerName     = "policy-controller"
	PermissionControllerName = "permission-controller"
)
