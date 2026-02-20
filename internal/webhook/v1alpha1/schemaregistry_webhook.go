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

"k8s.io/apimachinery/pkg/util/validation/field"
ctrl "sigs.k8s.io/controller-runtime"
logf "sigs.k8s.io/controller-runtime/pkg/log"
"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
)

// nolint:unused
var schemaregistrylog = logf.Log.WithName("schemaregistry-resource")

// SetupSchemaRegistryWebhookWithManager registers the webhook for SchemaRegistry in the manager.
func SetupSchemaRegistryWebhookWithManager(mgr ctrl.Manager) error {
return ctrl.NewWebhookManagedBy(mgr, &registryv1alpha1.SchemaRegistry{}).
WithValidator(&SchemaRegistryCustomValidator{}).
Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-registry-strimzi-io-v1alpha1-schemaregistry,mutating=false,failurePolicy=fail,sideEffects=None,groups=registry.strimzi.io,resources=schemaregistries,verbs=create;update,versions=v1alpha1,name=vschemaregistry-v1alpha1.kb.io,admissionReviewVersions=v1

// SchemaRegistryCustomValidator validates SchemaRegistry resources on create and update.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type SchemaRegistryCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SchemaRegistry.
func (v *SchemaRegistryCustomValidator) ValidateCreate(_ context.Context, obj *registryv1alpha1.SchemaRegistry) (admission.Warnings, error) {
schemaregistrylog.Info("Validation for SchemaRegistry upon creation", "name", obj.GetName())
return nil, validateSchemaRegistrySpec(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SchemaRegistry.
func (v *SchemaRegistryCustomValidator) ValidateUpdate(_ context.Context, _, newObj *registryv1alpha1.SchemaRegistry) (admission.Warnings, error) {
schemaregistrylog.Info("Validation for SchemaRegistry upon update", "name", newObj.GetName())
return nil, validateSchemaRegistrySpec(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SchemaRegistry.
func (v *SchemaRegistryCustomValidator) ValidateDelete(_ context.Context, obj *registryv1alpha1.SchemaRegistry) (admission.Warnings, error) {
schemaregistrylog.Info("Validation for SchemaRegistry upon deletion", "name", obj.GetName())
return nil, nil
}

// validateSchemaRegistrySpec performs validation shared between create and update.
func validateSchemaRegistrySpec(obj *registryv1alpha1.SchemaRegistry) error {
var allErrs field.ErrorList

if obj.Spec.URL == "" {
allErrs = append(allErrs, field.Required(
field.NewPath("spec", "url"),
"url must not be empty",
))
}

if obj.Spec.Timeout < 0 {
allErrs = append(allErrs, field.Invalid(
field.NewPath("spec", "timeout"),
obj.Spec.Timeout,
"timeout must be >= 0",
))
}

if obj.Spec.Auth != nil {
authPath := field.NewPath("spec", "auth")

switch obj.Spec.Auth.Type {
case registryv1alpha1.AuthTypeBasic:
if obj.Spec.Auth.BasicAuth == nil {
allErrs = append(allErrs, field.Required(
authPath.Child("basicAuth"),
"basicAuth must be set when auth type is BASIC",
))
} else if obj.Spec.Auth.BasicAuth.SecretRef == "" {
allErrs = append(allErrs, field.Required(
authPath.Child("basicAuth", "secretRef"),
"basicAuth.secretRef must not be empty",
))
}

case registryv1alpha1.AuthTypeBearer:
if obj.Spec.Auth.BearerAuth == nil {
allErrs = append(allErrs, field.Required(
authPath.Child("bearerAuth"),
"bearerAuth must be set when auth type is BEARER",
))
} else if obj.Spec.Auth.BearerAuth.SecretRef == "" {
allErrs = append(allErrs, field.Required(
authPath.Child("bearerAuth", "secretRef"),
"bearerAuth.secretRef must not be empty",
))
}

case registryv1alpha1.AuthTypeMTLS:
if obj.Spec.Auth.MTLS == nil {
allErrs = append(allErrs, field.Required(
authPath.Child("mtls"),
"mtls must be set when auth type is MTLS",
))
} else if obj.Spec.Auth.MTLS.CertSecretRef == "" {
allErrs = append(allErrs, field.Required(
authPath.Child("mtls", "certSecretRef"),
"mtls.certSecretRef must not be empty",
))
}
}
}

if len(allErrs) > 0 {
return allErrs.ToAggregate()
}
return nil
}
