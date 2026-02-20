/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
	schemaclient "github.com/honza/schema-strimzi-operator/internal/client"
)

// SchemaRegistryReconciler reconciles a SchemaRegistry object
type SchemaRegistryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemaregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemaregistries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemaregistries/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile performs a health check against the Schema Registry endpoint and
// updates the SchemaRegistry status with the current connectivity state.
// It re-queues every 5 minutes for periodic health monitoring.
func (r *SchemaRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var schemaRegistry registryv1alpha1.SchemaRegistry
	if err := r.Get(ctx, req.NamespacedName, &schemaRegistry); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Build the Schema Registry HTTP client from spec + secrets
	authConfig, err := loadAuthConfig(ctx, r.Client, &schemaRegistry)
	if err != nil {
		log.Error(err, "Failed to load auth config")
		return ctrl.Result{}, r.setConditionFailed(ctx, &schemaRegistry, "AuthLoadFailed", err.Error())
	}

	timeout := time.Duration(schemaRegistry.Spec.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	srClient, err := schemaclient.NewClient(
		schemaRegistry.Spec.URL,
		authConfig,
		timeout,
		schemaRegistry.Spec.InsecureSkipVerify,
	)
	if err != nil {
		log.Error(err, "Failed to create Schema Registry client")
		return ctrl.Result{}, r.setConditionFailed(ctx, &schemaRegistry, "ClientCreateFailed", err.Error())
	}

	// Health check
	healthErr := srClient.HealthCheck(ctx)

	// Re-fetch before status update to avoid conflicts
	if err := r.Get(ctx, req.NamespacedName, &schemaRegistry); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	schemaRegistry.Status.ObservedGeneration = schemaRegistry.Generation
	now := metav1.Now()
	schemaRegistry.Status.LastChecked = &now

	if healthErr != nil {
		log.Error(healthErr, "Schema Registry health check failed")
		meta.SetStatusCondition(&schemaRegistry.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ConnectionFailed",
			Message:            healthErr.Error(),
			ObservedGeneration: schemaRegistry.Generation,
		})
		schemaRegistry.Status.ConnectionStatus = "Unreachable"
	} else {
		log.Info("Schema Registry health check succeeded")
		meta.SetStatusCondition(&schemaRegistry.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            "Successfully connected to Schema Registry",
			ObservedGeneration: schemaRegistry.Generation,
		})
		schemaRegistry.Status.ConnectionStatus = "Connected"
	}

	if err := r.Status().Update(ctx, &schemaRegistry); err != nil {
		log.Error(err, "Failed to update SchemaRegistry status")
		return ctrl.Result{}, err
	}

	// Requeue periodically for ongoing health monitoring
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// setConditionFailed is a helper that sets a Failed status condition and updates the resource.
func (r *SchemaRegistryReconciler) setConditionFailed(ctx context.Context, sr *registryv1alpha1.SchemaRegistry, reason, message string) error {
	// Re-fetch to avoid conflicts
	if err := r.Get(ctx, client.ObjectKeyFromObject(sr), sr); err != nil {
		return client.IgnoreNotFound(err)
	}

	meta.SetStatusCondition(&sr.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sr.Generation,
	})
	sr.Status.ConnectionStatus = "Unreachable"

	return r.Status().Update(ctx, sr)
}

// findSchemaRegistriesForSecret maps a Secret change to SchemaRegistry reconcile requests.
func (r *SchemaRegistryReconciler) findSchemaRegistriesForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	srList := &registryv1alpha1.SchemaRegistryList{}
	if err := r.List(ctx, srList, client.InNamespace(secret.GetNamespace())); err != nil {
		return nil
	}
	var requests []reconcile.Request
	for _, sr := range srList.Items {
		if schemaRegistryReferencesSecret(&sr, secret.GetName()) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: sr.Namespace,
					Name:      sr.Name,
				},
			})
		}
	}
	return requests
}

// schemaRegistryReferencesSecret returns true if the SchemaRegistry references the given secret.
func schemaRegistryReferencesSecret(sr *registryv1alpha1.SchemaRegistry, secretName string) bool {
	if sr.Spec.Auth == nil {
		return false
	}
	switch sr.Spec.Auth.Type {
	case registryv1alpha1.AuthTypeBasic:
		return sr.Spec.Auth.BasicAuth != nil && sr.Spec.Auth.BasicAuth.SecretRef == secretName
	case registryv1alpha1.AuthTypeBearer:
		return sr.Spec.Auth.BearerAuth != nil && sr.Spec.Auth.BearerAuth.SecretRef == secretName
	case registryv1alpha1.AuthTypeMTLS:
		if sr.Spec.Auth.MTLS == nil {
			return false
		}
		return sr.Spec.Auth.MTLS.CertSecretRef == secretName || sr.Spec.Auth.MTLS.CASecretRef == secretName
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&registryv1alpha1.SchemaRegistry{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findSchemaRegistriesForSecret),
		).
		Named("schemaregistry").
		Complete(r)
}
