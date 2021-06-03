package controllers

import (
	"context"
	"crypto/x509"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// common error messages shared across controllers
const (
	failedStatusUpdate         = "failed to update object status"
	failedMarshalSpec          = "failed to marshal spec"
	failedGenerateRabbitClient = "failed to generate http rabbitClient"
	failedParseClusterRef      = "failed to retrieve cluster from reference"
	failedRetrieveSysCertPool  = "failed to retrieve system trusted certs"
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
	FederationControllerName        = "federation-controller"
	ShovelControllerName            = "shovel-controller"
)

func extractSystemCertPool(ctx context.Context, recorder record.EventRecorder, object runtime.Object) (*x509.CertPool, error) {
	logger := ctrl.LoggerFrom(ctx)

	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		recorder.Event(object, corev1.EventTypeWarning, "FailedUpdate", failedRetrieveSysCertPool)
		logger.Error(err, failedRetrieveSysCertPool)
	}
	return systemCertPool, err
}
