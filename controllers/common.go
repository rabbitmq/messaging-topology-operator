package controllers

// common error messages shared across controllers
const (
	failedStatusUpdate         = "Failed to update object status"
	failedMarshalSpec          = "Failed to marshal spec"
	failedGenerateRabbitClient = "Failed to generate http rabbitClient"
	noSuchRabbitDeletion       = "RabbitmqCluster is already gone: cannot find its connection secret"
)
