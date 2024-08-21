/*
Copyright 2024.

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
	"k8s.io/apimachinery/pkg/runtime"
)

// InstallationAccessTokenSpec defines the desired state of InstallationAccessToken
type InstallationAccessTokenSpec struct {
	// The GitHub App's ID
	AppID string `json:"appId"`

	// The Installation ID
	InstallationID string `json:"installationId"`

	// Optional template for customizing the generated resource
	// +kubebuilder:pruning:PreserveUnknownFields
	Template *runtime.RawExtension `json:"template,omitempty"`

	// Reference to the private key used for authentication
	PrivateKeyRef *PrivateKeyRef `json:"privateKeyRef,omitempty"`

	// Optional scope for the token
	Scope *Scope `json:"scope,omitempty"`
}

type PrivateKeyRef struct {
	// Name of the private key reference
	Name string `json:"name,omitempty"`

	// Optional namespace where the private key is stored
	Namespace string `json:"namespace,omitempty"`

	// Optional key identifier or value
	Key string `json:"key,omitempty"`
}

type Scope struct {
	// List of repository names that the token should have access to
	Repositories []string `json:"repositories,omitempty"`

	// List of repository IDs that the token should have access to
	RepositoryIDs []int `json:"repositoryIds,omitempty"`

	// The permissions granted to the user access token
	Permissions map[string]string `json:"permissions,omitempty"`
}

// InstallationAccessTokenStatus defines the observed state of InstallationAccessToken
type InstallationAccessTokenStatus struct {
	// List of current condition states
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Reference to the secret containing the token
	SecretRef SecretRef `json:"secretRef,omitempty"`

	// Token-specific information
	Token TokenInfo `json:"token,omitempty"`
}

type SecretRef struct {
	// Name of the secret
	Name string `json:"name"`

	// Namespace where the secret is stored
	Namespace string `json:"namespace,omitempty"`
}

type TokenInfo struct {
	// Expiration time of the token
	ExpiresAt metav1.Time `json:"expiresAt,omitempty"`

	// Permissions granted to the token
	Permissions map[string]string `json:"permissions,omitempty"`

	// How repositories are selected for this token
	RepositorySelection string `json:"repositorySelection,omitempty"`

	// List of repository names that the token has access to
	Repositories []string `json:"repositories,omitempty"`

	// List of repository IDs that the token has access to
	RepositoryIDs []int `json:"repositoryIds,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="App ID",type="string",JSONPath=".spec.appId"
// +kubebuilder:printcolumn:name="Installation ID",type="string",JSONPath=".spec.installationId"
// +kubebuilder:printcolumn:name="Private Key Name",type="string",JSONPath=".spec.privateKeyRef.name"
// +kubebuilder:printcolumn:name="Private Key Namespace",type="string",JSONPath=".spec.privateKeyRef.namespace"
// +kubebuilder:printcolumn:name="Secret Name",type="string",JSONPath=".status.secretRef.name"
// +kubebuilder:printcolumn:name="Secret Namespace",type="string",JSONPath=".status.secretRef.namespace"
// +kubebuilder:printcolumn:name="Token Expires At",type="date",JSONPath=".status.token.expiresAt"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Token",type="string",JSONPath=".status.conditions[?(@.type=='Token')].status"
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".status.conditions[?(@.type=='Secret')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// InstallationAccessToken is the Schema for the installationaccesstokens API
type InstallationAccessToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstallationAccessTokenSpec   `json:"spec,omitempty"`
	Status InstallationAccessTokenStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstallationAccessTokenList contains a list of InstallationAccessToken
type InstallationAccessTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstallationAccessToken `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstallationAccessToken{}, &InstallationAccessTokenList{})
}
