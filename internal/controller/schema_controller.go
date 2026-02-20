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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
        "k8s.io/apimachinery/pkg/types"
        ctrl "sigs.k8s.io/controller-runtime"
        "sigs.k8s.io/controller-runtime/pkg/client"
        "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
        "sigs.k8s.io/controller-runtime/pkg/handler"
        logf "sigs.k8s.io/controller-runtime/pkg/log"
        "sigs.k8s.io/controller-runtime/pkg/reconcile"

        registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
        schemaclient "github.com/honza/schema-strimzi-operator/internal/client"
)

const schemaFinalizer = "registry.strimzi.io/schema-finalizer"

type SchemaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemas/finalizers,verbs=update
// +kubebuilder:rbac:groups=registry.strimzi.io,resources=schemaregistries,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile registers the schema in Schema Registry or cleans it up when deleted.
// A finalizer ensures the subject is removed from the registry before the CR is deleted.
func (r *SchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var schema registryv1alpha1.Schema
	if err := r.Get(ctx, req.NamespacedName, &schema); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- Deletion path ---
	if !schema.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&schema, schemaFinalizer) {
			log.Info("Deleting schema subject from registry", "subject", schema.Spec.Subject)

			if err := r.deleteFromRegistry(ctx, &schema); err != nil {
				log.Error(err, "Failed to delete schema subject from registry")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&schema, schemaFinalizer)
			if err := r.Update(ctx, &schema); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// --- Add finalizer if missing ---
	if !controllerutil.ContainsFinalizer(&schema, schemaFinalizer) {
		controllerutil.AddFinalizer(&schema, schemaFinalizer)
		if err := r.Update(ctx, &schema); err != nil {
			return ctrl.Result{}, err
		}
		// Re-fetch after update
		if err := r.Get(ctx, req.NamespacedName, &schema); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	// --- Build Schema Registry client ---
	srClient, err := r.buildClient(ctx, &schema)
	if err != nil {
		log.Error(err, "Failed to build Schema Registry client")
		return ctrl.Result{RequeueAfter: time.Minute}, r.setConditionFailed(ctx, &schema, "ClientBuildFailed", err.Error())
	}

	// --- Register schema ---
	registerReq := schemaclient.RegisterSchemaRequest{
		Schema:     schema.Spec.Schema,
		SchemaType: string(schema.Spec.SchemaType),
		References: convertReferences(schema.Spec.References),
	}

	log.Info("Registering schema", "subject", schema.Spec.Subject, "type", schema.Spec.SchemaType)

	resp, err := srClient.RegisterSchema(ctx, schema.Spec.Subject, registerReq)
	if err != nil {
		log.Error(err, "Failed to register schema", "subject", schema.Spec.Subject)
		return ctrl.Result{RequeueAfter: time.Minute}, r.setConditionFailed(ctx, &schema, "RegistrationFailed", err.Error())
	}

	// --- Set compatibility level if specified ---
	if schema.Spec.CompatibilityLevel != "" {
		if err := srClient.SetCompatibility(ctx, schema.Spec.Subject, schema.Spec.CompatibilityLevel); err != nil {
			log.Error(err, "Failed to set compatibility level", "subject", schema.Spec.Subject, "level", schema.Spec.CompatibilityLevel)
			// Non-fatal: log but continue - schema is already registered
		}
	}

	// --- Update status ---
	// Re-fetch before status update to avoid conflicts
	if err := r.Get(ctx, req.NamespacedName, &schema); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now := metav1.Now()
	schema.Status.SchemaID = &resp.ID
	schema.Status.Version = &resp.Version
	schema.Status.RegisteredAt = &now
	schema.Status.ObservedGeneration = schema.Generation

	meta.SetStatusCondition(&schema.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Registered",
		Message:            fmt.Sprintf("Schema registered with ID %d, version %d", resp.ID, resp.Version),
		ObservedGeneration: schema.Generation,
	})

	if err := r.Status().Update(ctx, &schema); err != nil {
		log.Error(err, "Failed to update Schema status")
		return ctrl.Result{}, err
	}

	log.Info("Schema successfully registered", "subject", schema.Spec.Subject, "schemaID", resp.ID, "version", resp.Version)
	return ctrl.Result{}, nil
}

// buildClient constructs a Schema Registry HTTP client from the referenced SchemaRegistry CR.
func (r *SchemaReconciler) buildClient(ctx context.Context, schema *registryv1alpha1.Schema) (*schemaclient.SchemaRegistryClient, error) {
	registryNamespace := schema.Spec.RegistryRef.Namespace
	if registryNamespace == "" {
		registryNamespace = schema.Namespace
	}

	var schemaRegistry registryv1alpha1.SchemaRegistry
	if err := r.Get(ctx, client.ObjectKey{
		Name:      schema.Spec.RegistryRef.Name,
		Namespace: registryNamespace,
	}, &schemaRegistry); err != nil {
		return nil, fmt.Errorf("failed to get SchemaRegistry %q: %w", schema.Spec.RegistryRef.Name, err)
	}

	authConfig, err := loadAuthConfig(ctx, r.Client, &schemaRegistry)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(schemaRegistry.Spec.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return schemaclient.NewClient(
		schemaRegistry.Spec.URL,
		authConfig,
		timeout,
		schemaRegistry.Spec.InsecureSkipVerify,
	)
}

// deleteFromRegistry deletes the schema subject from Schema Registry during CR deletion.
func (r *SchemaReconciler) deleteFromRegistry(ctx context.Context, schema *registryv1alpha1.Schema) error {
	srClient, err := r.buildClient(ctx, schema)
	if err != nil {
		// If the registry itself is gone, we can still proceed with finalizer removal
		logf.FromContext(ctx).Info("Could not build client during deletion, skipping registry cleanup", "error", err.Error())
		return nil
	}

	return srClient.DeleteSubject(ctx, schema.Spec.Subject)
}

// setConditionFailed sets a failed status condition and updates the resource.
func (r *SchemaReconciler) setConditionFailed(ctx context.Context, schema *registryv1alpha1.Schema, reason, message string) error {
	// Re-fetch to avoid conflicts
	if err := r.Get(ctx, client.ObjectKeyFromObject(schema), schema); err != nil {
		return client.IgnoreNotFound(err)
	}

	meta.SetStatusCondition(&schema.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: schema.Generation,
	})

	return r.Status().Update(ctx, schema)
}

// convertReferences converts API schema references to client schema references.
func convertReferences(refs []registryv1alpha1.SchemaReference) []schemaclient.SchemaReference {
	result := make([]schemaclient.SchemaReference, len(refs))
	for i, ref := range refs {
		result[i] = schemaclient.SchemaReference{
			Name:    ref.Name,
			Subject: ref.Subject,
			Version: ref.Version,
		}
	}
	return result
}

// findSchemasForRegistry maps a SchemaRegistry change to Schema reconcile requests.
func (r *SchemaReconciler) findSchemasForRegistry(ctx context.Context, registry client.Object) []reconcile.Request {
        schemaList := &registryv1alpha1.SchemaList{}
        if err := r.List(ctx, schemaList, client.InNamespace(registry.GetNamespace())); err != nil {
                return nil
        }
        var requests []reconcile.Request
        for _, schema := range schemaList.Items {
                if schema.Spec.RegistryRef.Name == registry.GetName() {
                        requests = append(requests, reconcile.Request{
                                NamespacedName: types.NamespacedName{
                                        Namespace: schema.Namespace,
                                        Name:      schema.Name,
                                },
                        })
                }
        }
        return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
        return ctrl.NewControllerManagedBy(mgr).
                For(&registryv1alpha1.Schema{}).
                Watches(
                        &registryv1alpha1.SchemaRegistry{},
                        handler.EnqueueRequestsFromMapFunc(r.findSchemasForRegistry),
                ).
		Named("schema").
		Complete(r)
}
