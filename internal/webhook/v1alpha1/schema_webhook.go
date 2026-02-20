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

package v1alpha1

import (
"context"
"encoding/json"
"fmt"

"k8s.io/apimachinery/pkg/util/validation/field"
ctrl "sigs.k8s.io/controller-runtime"
logf "sigs.k8s.io/controller-runtime/pkg/log"
"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
)

// nolint:unused
var schemalog = logf.Log.WithName("schema-resource")

// SetupSchemaWebhookWithManager registers the webhook for Schema in the manager.
func SetupSchemaWebhookWithManager(mgr ctrl.Manager) error {
return ctrl.NewWebhookManagedBy(mgr, &registryv1alpha1.Schema{}).
WithValidator(&SchemaCustomValidator{}).
Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-registry-strimzi-io-v1alpha1-schema,mutating=false,failurePolicy=fail,sideEffects=None,groups=registry.strimzi.io,resources=schemas,verbs=create;update,versions=v1alpha1,name=vschema-v1alpha1.kb.io,admissionReviewVersions=v1

// SchemaCustomValidator validates Schema resources on create and update.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type SchemaCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Schema.
func (v *SchemaCustomValidator) ValidateCreate(_ context.Context, obj *registryv1alpha1.Schema) (admission.Warnings, error) {
schemalog.Info("Validation for Schema upon creation", "name", obj.GetName())
return nil, validateSchemaSpec(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Schema.
func (v *SchemaCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *registryv1alpha1.Schema) (admission.Warnings, error) {
schemalog.Info("Validation for Schema upon update", "name", newObj.GetName())

var allErrs field.ErrorList

// subject is immutable after creation
if oldObj.Spec.Subject != newObj.Spec.Subject {
allErrs = append(allErrs, field.Forbidden(
field.NewPath("spec", "subject"),
"subject is immutable and cannot be changed after creation",
))
}

// schemaType is immutable after creation
if oldObj.Spec.SchemaType != newObj.Spec.SchemaType {
allErrs = append(allErrs, field.Forbidden(
field.NewPath("spec", "schemaType"),
"schemaType is immutable and cannot be changed after creation",
))
}

if err := validateSchemaSpec(newObj); err != nil {
allErrs = append(allErrs, field.InternalError(field.NewPath("spec"), err))
}

if len(allErrs) > 0 {
return nil, allErrs.ToAggregate()
}
return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Schema.
func (v *SchemaCustomValidator) ValidateDelete(_ context.Context, obj *registryv1alpha1.Schema) (admission.Warnings, error) {
schemalog.Info("Validation for Schema upon deletion", "name", obj.GetName())
return nil, nil
}

// validateSchemaSpec performs validation shared between create and update.
func validateSchemaSpec(obj *registryv1alpha1.Schema) error {
var allErrs field.ErrorList

if obj.Spec.Subject == "" {
allErrs = append(allErrs, field.Required(
field.NewPath("spec", "subject"),
"subject must not be empty",
))
}

if obj.Spec.Schema == "" {
allErrs = append(allErrs, field.Required(
field.NewPath("spec", "schema"),
"schema content must not be empty",
))
}

if obj.Spec.RegistryRef.Name == "" {
allErrs = append(allErrs, field.Required(
field.NewPath("spec", "registryRef", "name"),
"registryRef.name must not be empty",
))
}

// AVRO and JSON schemas must be valid JSON
if (obj.Spec.SchemaType == registryv1alpha1.SchemaTypeAvro || obj.Spec.SchemaType == registryv1alpha1.SchemaTypeJSON) &&
obj.Spec.Schema != "" {
if !json.Valid([]byte(obj.Spec.Schema)) {
allErrs = append(allErrs, field.Invalid(
field.NewPath("spec", "schema"),
obj.Spec.Schema,
fmt.Sprintf("%s schema must be valid JSON", obj.Spec.SchemaType),
))
}
}

// Validate schema references
for i, ref := range obj.Spec.References {
refPath := field.NewPath("spec", "references").Index(i)
if ref.Name == "" {
allErrs = append(allErrs, field.Required(refPath.Child("name"), "reference name must not be empty"))
}
if ref.Subject == "" {
allErrs = append(allErrs, field.Required(refPath.Child("subject"), "reference subject must not be empty"))
}
if ref.Version < 1 {
allErrs = append(allErrs, field.Invalid(refPath.Child("version"), ref.Version, "reference version must be >= 1"))
}
}

if len(allErrs) > 0 {
return allErrs.ToAggregate()
}
return nil
}
