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

package flannel

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
	// FlannelClusterRoleName sets the name for the flannel ClusterRole
	FlannelClusterRoleName = "kubic:flannel"

	// FlannelClusterRoleNamePSP the PSP cluster role
	FlannelClusterRoleNamePSP = "kubic:psp:flannel"

	// FlannelServiceAccountName describes the name of the ServiceAccount for the flannel addon
	FlannelServiceAccountName = "kubic-flannel"

	// FlannelHealthPort Default health port for Glannel
	FlannelHealthPort = 8471
)

var (
	serviceAccount = v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      FlannelServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		},
	}

	clusterRole = rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: FlannelClusterRoleName,
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
			Name: FlannelClusterRoleName,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     FlannelClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      FlannelServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}

	clusterRoleBindingPSP = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: FlannelClusterRoleNamePSP,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "suse:kubic:psp:privileged",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      FlannelServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}
)

func init() {
	// self-register in the CNI plugins registry
	cni.Registry.Register("flannel", EnsureFlannelAddon)
}

// EnsureFlannelAddon creates the flannel addons
func EnsureFlannelAddon(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	if err := createServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating flannel service account: %v", err)
	}

	var flannelConfigMapBytes, flannelDaemonSetBytes []byte
	flannelConfigMapBytes, err := kubeadmutil.ParseTemplate(FlannelConfigMap19,
		struct {
			Network string
			Backend string
		}{
			cfg.Network.PodSubnet,
			"vxlan", // TODO: replace by some config arg
		})

	if err != nil {
		return fmt.Errorf("error when parsing flannel configmap template: %v", err)
	}

	flannelDaemonSetBytes, err = kubeadmutil.ParseTemplate(FlannelDaemonSet19,
		struct {
			Image          string
			LogLevel       int
			HealthzPort    int
			ConfDir        string
			BinDir         string
			ServiceAccount string
		}{
			cfg.Network.Cni.Image,
			1, // TODO: replace by some config arg
			FlannelHealthPort,
			cfg.Network.Cni.ConfDir,
			cfg.Network.Cni.BinDir,
			FlannelServiceAccountName,
		})

	if err != nil {
		return fmt.Errorf("error when parsing flannel daemonset template: %v", err)
	}

	if err := createFlannelAddon(flannelConfigMapBytes, flannelDaemonSetBytes, client); err != nil {
		return err
	}

	if err := createRBACRules(client, cfg.Features.PSP); err != nil {
		return fmt.Errorf("error when creating flannel RBAC rules: %v", err)
	}

	glog.V(1).Infof("[kubic] installed flannel CNI driver")
	return nil
}

// CreateServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func createServiceAccount(client clientset.Interface) error {
	return apiclient.CreateOrUpdateServiceAccount(client, &serviceAccount)
}

func createFlannelAddon(configMapBytes, daemonSetbytes []byte, client clientset.Interface) error {
	flannelConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configMapBytes, flannelConfigMap); err != nil {
		return fmt.Errorf("unable to decode flannel configmap %v", err)
	}

	// Create the ConfigMap for flannel or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, flannelConfigMap); err != nil {
		return err
	}

	flannelDaemonSet := &apps.DaemonSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), daemonSetbytes, flannelDaemonSet); err != nil {
		return fmt.Errorf("unable to decode flannel daemonset %v", err)
	}

	// Create the DaemonSet for flannel or update it in case it already exists
	return apiclient.CreateOrUpdateDaemonSet(client, flannelDaemonSet)
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
