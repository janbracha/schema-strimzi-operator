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

// AuthType defines the type of authentication
// +kubebuilder:validation:Enum=NONE;BASIC;BEARER;MTLS
type AuthType string

const (
	AuthTypeNone   AuthType = "NONE"
	AuthTypeBasic  AuthType = "BASIC"
	AuthTypeBearer AuthType = "BEARER"
	AuthTypeMTLS   AuthType = "MTLS"
)

// BasicAuthConfig holds basic authentication credentials
type BasicAuthConfig struct {
	// SecretRef references a secret containing username and password
	// Expected keys: username, password
	// +required
	SecretRef string `json:"secretRef"`
}

// BearerAuthConfig holds bearer token authentication
type BearerAuthConfig struct {
	// SecretRef references a secret containing bearer token
	// Expected key: token
	// +required
	SecretRef string `json:"secretRef"`
}

// MTLSConfig holds mutual TLS configuration
type MTLSConfig struct {
	// CertSecretRef references a secret containing client certificate and key
	// Expected keys: tls.crt, tls.key
	// +required
	CertSecretRef string `json:"certSecretRef"`

	// CASecretRef references a secret containing CA certificate
	// Expected key: ca.crt
	// +optional
	CASecretRef string `json:"caSecretRef,omitempty"`
}

// AuthConfig defines authentication configuration for Schema Registry
type AuthConfig struct {
	// Type of authentication to use
	// +required
	// +kubebuilder:default=NONE
	Type AuthType `json:"type"`

	// BasicAuth configuration (used when type is BASIC)
	// +optional
	BasicAuth *BasicAuthConfig `json:"basicAuth,omitempty"`

	// BearerAuth configuration (used when type is BEARER)
	// +optional
	BearerAuth *BearerAuthConfig `json:"bearerAuth,omitempty"`

	// MTLS configuration (used when type is MTLS)
	// +optional
	MTLS *MTLSConfig `json:"mtls,omitempty"`
}

// SchemaRegistrySpec defines the desired state of SchemaRegistry
type SchemaRegistrySpec struct {
	// URL is the endpoint URL of the Schema Registry
	// +required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// Auth defines authentication configuration
	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`

	// InsecureSkipVerify controls whether to skip TLS certificate verification
	// +optional
	// +kubebuilder:default=false
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// Timeout for requests to Schema Registry (in seconds)
	// +optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	Timeout int `json:"timeout,omitempty"`
}

// SchemaRegistryStatus defines the observed state of SchemaRegistry.
type SchemaRegistryStatus struct {
	// ConnectionStatus indicates whether the registry is reachable
	// +optional
	ConnectionStatus string `json:"connectionStatus,omitempty"`

	// LastChecked is the timestamp of the last connectivity check
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed SchemaRegistry Spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the SchemaRegistry resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the schema registry is reachable and operational
	// - "Progressing": the schema registry connection is being established
	// - "Failed": connection to the schema registry failed
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SchemaRegistry is the Schema for the schemaregistries API
type SchemaRegistry struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SchemaRegistry
	// +required
	Spec SchemaRegistrySpec `json:"spec"`

	// status defines the observed state of SchemaRegistry
	// +optional
	Status SchemaRegistryStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SchemaRegistryList contains a list of SchemaRegistry
type SchemaRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SchemaRegistry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SchemaRegistry{}, &SchemaRegistryList{})
}
