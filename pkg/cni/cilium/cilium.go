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

package cilium

import (
	"fmt"

	"github.com/golang/glog"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"

	"github.com/kubic-project/kubic-init/pkg/cni"
	"github.com/kubic-project/kubic-init/pkg/config"
)

const (
	// CiliumClusterRoleName sets the name for the cilium ClusterRole
	CiliumClusterRoleName = "kubic:cilium"

	// CiliumClusterRoleNamePSP the PSP cluster role
	CiliumClusterRoleNamePSP = "kubic:psp:cilium"

	// CiliumServiceAccountName describes the name of the ServiceAccount for the cilium addon
	CiliumServiceAccountName = "cilium"

	// CiliumHealthPort Default health port for Glannel
	CiliumHealthPort = 8471
)

var (
	serviceAccount = v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CiliumServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		},
	}

	clusterRole = rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: CiliumClusterRoleName,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/status"},
				Verbs:     []string{"patch"},
			},
		},
	}

	clusterRoleBinding = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CiliumClusterRoleName,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     CiliumClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      CiliumServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}

	clusterRoleBindingPSP = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CiliumClusterRoleNamePSP,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "suse:kubic:psp:privileged",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      CiliumServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}
)

func init() {
	// self-register in the CNI plugins registry
	cni.Registry.Register("cilium", EnsureCiliumAddon)
}

// EnsureCiliumAddon creates the cilium addons
func EnsureCiliumAddon(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	if err := createServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating cilium service account: %v", err)
	}

	var ciliumConfigMapBytes, ciliumDaemonSetBytes []byte
	ciliumConfigMapBytes, err := kubeadmutil.ParseTemplate(CiliumConfigMap,
		struct {
			Network string
			Backend string
		}{
			cfg.Network.PodSubnet,
			"vxlan", // TODO: replace by some config arg
		})

	if err != nil {
		return fmt.Errorf("error when parsing cilium configmap template: %v", err)
	}

	ciliumDaemonSetBytes, err = kubeadmutil.ParseTemplate(CiliumDaemonSet,
		struct {
			Image              string
			LogLevel           int
			HealthzPort        int
			ConfDir            string
			BinDir             string
			ServiceAccountName string
		}{
			cfg.Network.Cni.Image,
			1, // TODO: replace by some config arg
			CiliumHealthPort,
			cfg.Network.Cni.ConfDir,
			cfg.Network.Cni.BinDir,
			CiliumServiceAccountName,
		})

	if err != nil {
		return fmt.Errorf("error when parsing cilium daemonset template: %v", err)
	}

	if err := createCiliumAddon(ciliumConfigMapBytes, ciliumDaemonSetBytes, client); err != nil {
		return err
	}

	if err := createRBACRules(client, cfg.Features.PSP); err != nil {
		return fmt.Errorf("error when creating cilium RBAC rules: %v", err)
	}

	glog.V(1).Infof("[kubic] installed cilium CNI driver")
	return nil
}

// CreateServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func createServiceAccount(client clientset.Interface) error {
	return apiclient.CreateOrUpdateServiceAccount(client, &serviceAccount)
}

func createCiliumAddon(configMapBytes, daemonSetbytes []byte, client clientset.Interface) error {
	ciliumConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configMapBytes, ciliumConfigMap); err != nil {
		return fmt.Errorf("unable to decode cilium configmap %v", err)
	}

	// Create the ConfigMap for cilium or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, ciliumConfigMap); err != nil {
		return err
	}

	ciliumDaemonSet := &apps.DaemonSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), daemonSetbytes, ciliumDaemonSet); err != nil {
		return fmt.Errorf("unable to decode cilium daemonset %v", err)
	}

	// Create the DaemonSet for cilium or update it in case it already exists
	return apiclient.CreateOrUpdateDaemonSet(client, ciliumDaemonSet)
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
