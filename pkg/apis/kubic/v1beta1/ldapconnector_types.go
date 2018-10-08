/*
 * Copyright 2018 SUSE LINUX GmbH, Nuernberg, Germany..
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LDAPUserSpec struct {
	BaseDN string `json:"baseDn,omitempty"`

	Filter string `json:"filter,omitempty"`

	Username string `json:"username,omitempty"`

	IdAttr string `json:"idAttr,omitempty"`

	// +optional
	EmailAttr string `json:"emailAttr,omitempty"`

	// +optional
	NameAttr string `json:"nameAttr,omitempty"`
}

type LDAPGroupSpec struct {
	BaseDN string `json:"baseDn,omitempty"`

	Filter string `json:"filter,omitempty"`

	UserAttr string `json:"userAttr,omitempty"`

	GroupAttr string `json:"groupAttr,omitempty"`

	// +optional
	NameAttr string `json:"nameAttr,omitempty"`
}

// LDAPConnectorSpec defines the desired state of LDAPConnector
type LDAPConnectorSpec struct {
	Name string `json:"name,omitempty"`

	Id string `json:"id,omitempty"`

	Server string `json:"server,omitempty"`

	// +optional
	BindDN string `json:"bindDn,omitempty"`

	// +optional
	BindPW string `json:"bindPw,omitempty"`

	// +optional
	UsernamePrompt string `json:"usernamePrompt,omitempty"`

	// +optional
	StartTLS bool `json:"startTLS,omitempty"`

	// +optional
	RootCAData string `json:"rootCAData,omitempty"`

	// +optional
	User LDAPUserSpec `json:"user,omitempty"`

	// +optional
	Group LDAPGroupSpec `json:"group,omitempty"`
}

// LDAPConnectorStatus defines the observed state of LDAPConnector
type LDAPConnectorStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// LDAPConnector is the Schema for the ldapconnectors API
// +k8s:openapi-gen=true
type LDAPConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LDAPConnectorSpec   `json:"spec,omitempty"`
	Status LDAPConnectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// LDAPConnectorList contains a list of LDAPConnector
type LDAPConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LDAPConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LDAPConnector{}, &LDAPConnectorList{})
}
