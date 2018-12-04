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
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/golang/glog"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	certutil "k8s.io/client-go/util/cert"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kubic-project/kubic-init/pkg/cni"
	"github.com/kubic-project/kubic-init/pkg/config"
)

const (
	// CiliumClusterRoleName sets the name for the cilium ClusterRole
	CiliumClusterRoleName = "cilium"

	// CiliumClusterRoleNamePSP the PSP cluster role
	CiliumClusterRoleNamePSP = "kubic:psp:cilium"

	// CiliumServiceAccountName describes the name of the ServiceAccount for the cilium addon
	CiliumServiceAccountName = "cilium"
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
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"networkpolicies"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "services", "nodes", "endpoints", "componentstatuses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "nodes"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"networkpolicies", "thirdpartyresources", "ingresses"},
				Verbs:     []string{"get", "list", "watch", "create"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"create", "get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"cilium.io"},
				Resources: []string{"ciliumnetworkpolicies", "ciliumnetworkpolicies/status", "ciliumendpoints", "ciliumendpoints/status"},
				Verbs:     []string{"*"},
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
			{
				Kind: "Group",
				Name: "system:nodes",
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

	ciliumBaseName   = "cilium-etcd-client"
	ciliumCertConfig = certutil.Config{
		CommonName: "cilium-etcd-client",
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
)

func init() {
	// self-register in the CNI plugins registry
	cni.Registry.Register("cilium", EnsureCiliumAddon)
}

// EnsureCiliumAddon creates the cilium addons
func EnsureCiliumAddon(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	etcdDir := strings.Join([]string{cfg.Certificates.Directory, "/etcd"}, "")
	var ciliumConfigMapBytes, ciliumDaemonSetBytes []byte

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(etcdDir, "ca")
	if err != nil {
		fmt.Printf("etcd generation retrieval failed %s", err)
	}

	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, &ciliumCertConfig)
	if err != nil {
		fmt.Printf("failed to create etcd client certificate for cilium %s", err)
	}

	pkiutil.WriteCertAndKey(etcdDir, ciliumBaseName, cert, key)
	if err := createServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating cilium service account: %v", err)
	}

	etcdServer, err := cfg.GetBindIP()
	if err != nil {
		return fmt.Errorf("failed to retrieve etcd server IP %s", err)
	}

	ciliumConfigMapBytes, err = kubeadmutil.ParseTemplate(CiliumConfigMap,
		struct {
			EtcdServer string
		}{
			etcdServer.String(),
		})

	if err != nil {
		return fmt.Errorf("error when parsing cilium configmap template: %v", err)
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cilium-secret",
			Namespace: "kube-system",
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			v1.TLSCertKey:       certutil.EncodeCertPEM(cert),
			v1.TLSPrivateKeyKey: certutil.EncodePrivateKeyPEM(key),
			"ca.crt":            certutil.EncodeCertPEM(caCert),
		},
	}
	if err = apiclient.CreateOrUpdateSecret(client, secret); err != nil {
		fmt.Printf("Cert failed %v", err)
	}

	ciliumDaemonSetBytes, err = kubeadmutil.ParseTemplate(CiliumDaemonSet,
		struct {
			Image   string
			ConfDir string
			BinDir  string
		}{
			cfg.Network.Cni.Image,
			cfg.Network.Cni.ConfDir,
			cfg.Network.Cni.BinDir,
		})

	if err != nil {
		return fmt.Errorf("error when parsing cilium daemonset template: %v", err)
	}

	if err := createRBACRules(client, cfg.Features.PSP); err != nil {
		return fmt.Errorf("error when creating flannel RBAC rules: %v", err)
	}

	if err := createCiliumAddon(ciliumConfigMapBytes, ciliumDaemonSetBytes, client); err != nil {
		return err
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
	if err := apiclient.CreateOrUpdateDaemonSet(client, ciliumDaemonSet); err != nil {
		return err
	}

	return nil

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
