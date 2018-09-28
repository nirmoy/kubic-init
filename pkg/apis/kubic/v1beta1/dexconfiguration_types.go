/*
Copyright 2018 SUSE LINUX GmbH, Nuernberg, Germany..

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DexConfigurationSpec defines the desired state of DexConfiguration
type DexConfigurationSpec struct {
	// the NodePort used y the Dex server
	// +optional
	NodePort int `json:"nodePort,omitempty"`

	// The image used for Dex
	// +optional
	Image string `json:"image,omitempty"`

	// Use an (already existing) certificate for the Dex service
	// +optional
	Certificate corev1.SecretReference `json:"certificate,omitempty"`

	// TODO: maybe this should be a property of the LDAPConnector
	// +optional
	AdminGroup string `json:"adminGroup,omitempty"`
}

// DexConfigurationStatus defines the observed state of DexConfiguration
type DexConfigurationStatus struct {
	// Config is the (maybe namespaced) name of the ConfigMap
	Config string `json:"config,omitempty"`

	// Current deployment
	Deployment string `json:"deployment,omitempty"`

	// GeneratedCertificate is the certificate automatically generated for the Dex service
	// It will be empty when using the certificate provided in Spec.Certificate
	// It will be automatically removed when removing the DexConfiguration
	GeneratedCertificate corev1.SecretReference `json:"generatedCertificate,omitempty"`

	// Shared, static passwords generated for other services
	StaticPasswords map[string]corev1.SecretReference `json:"staticPasswords,omitempty"`

	// Number of connectors currently installed
	NumConnectors int `json:"numConnectors,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// DexConfiguration is the Schema for the dexconfigurations API
// +k8s:openapi-gen=true
type DexConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DexConfigurationSpec   `json:"spec,omitempty"`
	Status DexConfigurationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// DexConfigurationList contains a list of DexConfiguration
type DexConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DexConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DexConfiguration{}, &DexConfigurationList{})
}
