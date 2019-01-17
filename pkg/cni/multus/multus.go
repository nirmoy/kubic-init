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

package multus

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
	"github.com/kubic-project/kubic-init/pkg/cni"
	"github.com/kubic-project/kubic-init/pkg/cni/cilium"
	"github.com/kubic-project/kubic-init/pkg/cni/flannel"
	"github.com/kubic-project/kubic-init/pkg/config"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	"github.com/kubic-project/kubic-init/pkg/loader"
	"github.com/kubic-project/kubic-init/pkg/util"
)

const (
	// multusClusterRoleName sets the name for the multus ClusterRole
	multusClusterRoleName = "kubic:multus"

	// multusClusterRoleNamePSP the PSP cluster role
	multusClusterRoleNamePSP = "kubic:psp:multus"

	// multusServiceAccountName describes the name of the ServiceAccount for the multus addon
	multusServiceAccountName = "kubic-multus"

	// multusNetworkName describes the name of the cni config
	multusNetworkName = "multus-cni-network"
)

var (
	serviceAccount = v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      multusServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		},
	}

	clusterRole = rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: multusClusterRoleName,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"k8s.cni.cncf.io"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/status"},
				Verbs:     []string{"get", "update"},
			},
		},
	}

	clusterRoleBinding = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: multusClusterRoleName,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     multusClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      multusServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}

	clusterRoleBindingPSP = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: multusClusterRoleNamePSP,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "suse:kubic:psp:privileged",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      multusServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}

	customResourceDefinition = apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "network-attachment-definitions.k8s.cni.cncf.io",
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   "k8s.cni.cncf.io",
			Version: "v1",
			Scope:   "Namespaced",
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural:     "network-attachment-definitions",
				Singular:   "network-attachment-definition",
				Kind:       "NetworkAttachmentDefinition",
				ShortNames: []string{"net-attach-def"},
			},
			Validation: &apiextensionsv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextensionsv1beta1.JSONSchemaProps{
					Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
						"spec": apiextensionsv1beta1.JSONSchemaProps{
							Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
								"config": apiextensionsv1beta1.JSONSchemaProps{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
	}
)

type multusCniStruct struct {
	Name       string                   `json:"name,omitempty"`
	Type       string                   `json:"type,omitempty"`
	Kubeconfig string                   `json:"kubeconfig"`
	Delegates  []map[string]interface{} `json:"delegates"`
}

func init() {
	// self-register in the CNI plugins registry
	cni.Registry.Register("multus", EnsuremultusAddon)
}

// EnsuremultusAddon creates the multus addons
func EnsuremultusAddon(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	if err := createServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating multus service account: %v", err)
	}

	multusConfigMap := ""
	switch cfg.Network.Cni.Driver {
	case "cilium":
		multusConfigMap = cilium.CiliumCniConfigMap
	case "flannel":
		multusConfigMap = flannel.FlannelConfigMap19
	}

	var multusConfigMapBytes []byte
	multusConfigMapBytes, err := kubeadmutil.ParseTemplate(multusConfigMap,
		struct {
			Network    string
			Backend    string
			KubeConfig string
		}{
			cfg.Network.PodSubnet,
			"vxlan",                         // TODO: replace by some config arg
			kubiccfg.DefaultKubicKubeconfig, //TODO create kubeconfig for multus
		})

	if err != nil {
		return fmt.Errorf("error when parsing multus configmap template: %v", err)
	}

	if err := createmultusAddon(multusConfigMapBytes, client, cfg.Network.Cni.Driver); err != nil {
		return fmt.Errorf("error when creating multus addons: %v", err)
	}

	if err := createRBACRules(client, cfg.Features.PSP); err != nil {
		return fmt.Errorf("error when creating multus RBAC rules: %v", err)
	}

	kubeconfig, err := kubicclient.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to create kubic client: %v", err)
	}

	multusCRD := loader.CrdsSet{}
	multusCRD[util.NamespacedObjToString(&customResourceDefinition)] = &customResourceDefinition

	if err := loader.CreateCRDs(kubeconfig, multusCRD); err != nil {
		return fmt.Errorf("failed to create multus CRD: %v", err)
	}

	glog.V(1).Infof("[kubic] installed multus CNI driver")
	return nil
}

// CreateServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func createServiceAccount(client clientset.Interface) error {
	return apiclient.CreateOrUpdateServiceAccount(client, &serviceAccount)
}

func createmultusAddon(configMapBytes []byte, client clientset.Interface, driver string) error {
	delegateConfigMap := &v1.ConfigMap{}

	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configMapBytes, delegateConfigMap); err != nil {
		return fmt.Errorf("unable to decode multus configmap %v", err)
	}

	delegateCniJSON := []byte(delegateConfigMap.Data["cni-conf.json"])
	var multusCniJSON map[string]interface{}
	json.Unmarshal(delegateCniJSON, &multusCniJSON)
	multusCniConfig := multusCniStruct{
		Name:       multusNetworkName,
		Type:       "multus",
		Kubeconfig: kubiccfg.DefaultKubicKubeconfig,
		Delegates:  []map[string]interface{}{multusCniJSON},
	}
	marshaledCniJSON, _ := json.Marshal(multusCniConfig)
	multusConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cni-config",
			Namespace: metav1.NamespaceSystem,
			Labels: map[string]string{
				"tier": "node",
				"app":  "multus",
			},
		},
		Data: map[string]string{
			"cni-conf.json": string(marshaledCniJSON),
		},
	}

	if driver == "flannel" {
		multusConfigMap.Data["net-conf.json"] = delegateConfigMap.Data["net-conf.json"]
	}

	// Create the ConfigMap for multus or update it in case it already exists
	return apiclient.CreateOrUpdateConfigMap(client, multusConfigMap)

}

// CreateRBACRules creates the essential RBAC rules for a minimally set-up cluster
func createRBACRules(client clientset.Interface, psp bool) error {
	var err error

	if err = apiclient.CreateOrUpdateClusterRole(client, &clusterRole); err != nil {
		return err
	}

	if err = apiclient.CreateOrUpdateClusterRoleBinding(client, &clusterRoleBinding); err != nil {
		return err
	}

	if psp {
		if err = apiclient.CreateOrUpdateClusterRoleBinding(client, &clusterRoleBindingPSP); err != nil {
			return err
		}
	}

	return nil
}
