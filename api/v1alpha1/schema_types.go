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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SchemaType defines the type of schema
// +kubebuilder:validation:Enum=AVRO;JSON;PROTOBUF
type SchemaType string

const (
	SchemaTypeAvro     SchemaType = "AVRO"
	SchemaTypeJSON     SchemaType = "JSON"
	SchemaTypeProtobuf SchemaType = "PROTOBUF"
)

// SchemaReference represents a reference to another schema
type SchemaReference struct {
	// Name of the referenced schema subject
	// +required
	Name string `json:"name"`

	// Subject of the referenced schema
	// +required
	Subject string `json:"subject"`

	// Version of the referenced schema
	// +required
	Version int `json:"version"`
}

// SchemaRegistryRef references a Schema Registry endpoint
type SchemaRegistryRef struct {
	// Name of the schema registry configuration
	// +required
	Name string `json:"name"`

	// Namespace where the schema registry configuration is located
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// SchemaSpec defines the desired state of Schema
type SchemaSpec struct {
	// Subject is the name under which the schema will be registered
	// +required
	// +kubebuilder:validation:MinLength=1
	Subject string `json:"subject"`

	// SchemaType defines the type of schema (AVRO, JSON, PROTOBUF)
	// +required
	// +kubebuilder:default=AVRO
	SchemaType SchemaType `json:"schemaType"`

	// Schema is the actual schema definition
	// +required
	// +kubebuilder:validation:MinLength=1
	Schema string `json:"schema"`

	// References to other schemas (for nested/imported schemas)
	// +optional
	References []SchemaReference `json:"references,omitempty"`

	// RegistryRef references the Schema Registry endpoint configuration
	// +required
	RegistryRef SchemaRegistryRef `json:"registryRef"`

	// CompatibilityLevel defines the compatibility checking mode
	// Valid values: BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE, NONE
	// +optional
	// +kubebuilder:validation:Enum=BACKWARD;BACKWARD_TRANSITIVE;FORWARD;FORWARD_TRANSITIVE;FULL;FULL_TRANSITIVE;NONE
	CompatibilityLevel string `json:"compatibilityLevel,omitempty"`
}

// SchemaStatus defines the observed state of Schema.
type SchemaStatus struct {
	// SchemaID is the ID assigned by the Schema Registry
	// +optional
	SchemaID *int `json:"schemaId,omitempty"`

	// Version is the version number of the registered schema
	// +optional
	Version *int `json:"version,omitempty"`

	// RegisteredAt is the timestamp when the schema was registered
	// +optional
	RegisteredAt *metav1.Time `json:"registeredAt,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed Schema Spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the Schema resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the schema is successfully registered in the registry
	// - "Progressing": the schema is being registered or updated
	// - "Failed": the schema registration failed
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Schema is the Schema for the schemas API
type Schema struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Schema
	// +required
	Spec SchemaSpec `json:"spec"`

	// status defines the observed state of Schema
	// +optional
	Status SchemaStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SchemaList contains a list of Schema
type SchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Schema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Schema{}, &SchemaList{})
}
