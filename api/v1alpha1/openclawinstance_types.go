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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ImageSpec struct {
	// +kubebuilder:default="ghcr.io/openclaw/openclaw"
	Repository string `json:"repository,omitempty"`
	// +kubebuilder:default="latest"
	Tag        string            `json:"tag,omitempty"`
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type ConfigSpec struct {
	// Reference to an external ConfigMap
	// +optional
	ConfigMapRef *ConfigMapKeyRef `json:"configMapRef,omitempty"`

	// Inline JSON config (operator creates a managed ConfigMap)
	// +optional
	Raw *apiextensionsv1.JSON `json:"raw,omitempty"`

	// How config is applied to the PVC: "overwrite" or "merge"
	// +kubebuilder:validation:Enum=overwrite;merge
	// +kubebuilder:default=merge
	MergeMode string `json:"mergeMode,omitempty"`
}

type ConfigMapKeyRef struct {
	Name string `json:"name"`
	// +kubebuilder:default="openclaw.json"
	Key string `json:"key,omitempty"`
}

type StorageSpec struct {
	// +kubebuilder:default="10Gi"
	Size string `json:"size,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

type OllamaSpec struct {
	Enabled   bool                        `json:"enabled"`
	Image     ImageSpec                   `json:"image,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Storage for model data (separate PVC)
	Storage OllamaStorageSpec `json:"storage,omitempty"`
}
type OllamaStorageSpec struct {
	// +kubebuilder:default="50Gi"
	Size             string  `json:"size,omitempty"`
	StorageClassName *string `json:"storageClassName,omitempty"`
}
type CLISpec struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`
}

// OpenClawInstanceSpec defines the desired state of OpenClawInstance.
type OpenClawInstanceSpec struct {

	// Image for the main OpenClaw container
	Image ImageSpec `json:"image,omitempty"`
	// Config for openclaw.json
	// +optional
	Config *ConfigSpec `json:"config,omitempty"`
	// EnvFrom injects env vars from Secrets/ConfigMaps
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// Env sets individual environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// Resources for the main container
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Storage configures the data PVC
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`
	// Ollama deployment for local LLM inference
	// +optional
	Ollama *OllamaSpec `json:"ollama,omitempty"`
	// Tools to install via apt-get in an init container
	// +kubebuilder:validation:MaxItems=30
	// +optional
	Tools []string `json:"tools,omitempty"`

	// CLI sidecar for interactive TUI access
	// +optional
	CLI *CLISpec `json:"cli,omitempty"`
}

// OpenClawInstanceStatus defines the observed state of OpenClawInstance.
type OpenClawInstanceStatus struct {
	// Phase: Pending, Provisioning, Running, Failed, Terminating
	Phase string `json:"phase,omitempty"`
	// Standard conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// In-cluster gateway endpoint
	GatewayEndpoint string `json:"gatewayEndpoint,omitempty"`
	// Last successful reconciliation
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
	// Generation observed by controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.status.gatewayEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpenClawInstance is the Schema for the openclawinstances API.
type OpenClawInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawInstanceSpec   `json:"spec,omitempty"`
	Status OpenClawInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenClawInstanceList contains a list of OpenClawInstance.
type OpenClawInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenClawInstance{}, &OpenClawInstanceList{})
}
