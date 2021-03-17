package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var logger = logf.Log.WithName("binding-webhook")

func (r *Binding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1alpha1-binding,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=bindings,versions=v1alpha1,name=vbinding.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Binding{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Binding) ValidateCreate() error {
	logger.Info("validate create", "name", r.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Binding) ValidateUpdate(old runtime.Object) error {
	logger.Info("validate update", "name", r.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Binding) ValidateDelete() error {
	logger.Info("validate delete", "name", r.Name)

	return nil
}
