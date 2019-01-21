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
	"path/filepath"

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
	CiliumClusterRoleName = "kubic:cilium"

	// CiliumClusterRoleNamePSP the PSP cluster role
	CiliumClusterRoleNamePSP = "kubic:psp:cilium"

	// CiliumServiceAccountName describes the name of the ServiceAccount for the cilium addon
	CiliumServiceAccountName = "kubic-cilium"

	//CiliumCertSecret the secret name to store tls credential
	CiliumCertSecret = "cilium-secret"
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
	etcdDir := filepath.Join(cfg.Certificates.Directory, "etcd")
	var ciliumCniConfigMapBytes, ciliumEtcdConfigMapBytes, ciliumDaemonSetBytes []byte

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(etcdDir, "ca")
	if err != nil {
		return fmt.Errorf("etcd generation retrieval failed %v", err)
	}

	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, &ciliumCertConfig)
	if err != nil {
		return fmt.Errorf("error when creating etcd client certificate for cilium %v", err)
	}

	if err := pkiutil.WriteCertAndKey(etcdDir, ciliumBaseName, cert, key); err != nil {
		return fmt.Errorf("error when creating cilium etcd certificate: %v", err)
	}

	if err := createServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating cilium service account: %v", err)
	}

	etcdServer, err := cfg.GetBindIP()
	if err != nil {
		return fmt.Errorf("error when trying to retrieve etcd server IP %v", err)
	}
	ciliumCniConfigMapBytes = nil
	if !cfg.Network.MultipleCni {
		ciliumCniConfigMapBytes, err = kubeadmutil.ParseTemplate(CiliumCniConfigMap,
			struct {
				EtcdServer string
			}{
				etcdServer.String(),
			})

		if err != nil {
			return fmt.Errorf("error when parsing cilium cni configmap template: %v", err)
		}
	}
	ciliumEtcdConfigMapBytes, err = kubeadmutil.ParseTemplate(CiliumEtcdConfigMap,
		struct {
			EtcdServer string
		}{
			etcdServer.String(),
		})

	if err != nil {
		return fmt.Errorf("error when parsing cilium etcd configmap template: %v", err)
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CiliumCertSecret,
			Namespace: metav1.NamespaceSystem,
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			v1.TLSCertKey:       certutil.EncodeCertPEM(cert),
			v1.TLSPrivateKeyKey: certutil.EncodePrivateKeyPEM(key),
			"ca.crt":            certutil.EncodeCertPEM(caCert),
		},
	}
	if err = apiclient.CreateOrUpdateSecret(client, secret); err != nil {
		return fmt.Errorf("error when creating cilium secret  %v", err)
	}

	image := cfg.Network.Cni.Image
	if len(image) == 0 {
		image = config.DefaultCiliumImage
	}

	glog.V(1).Infof("[kubic] using %s as cni image", image)

	daemonSetName := "cilium"
	confName := "10-cilium.conf"
	enableBPF := "true"
	if cfg.Network.MultipleCni {
		daemonSetName = "cilium-with-multus"
		confName = "10-cilium-multus.conf"
	}

	if cfg.Runtime.Engine == "crio" {
		enableBPF = ""
	}
	ciliumDaemonSetBytes, err = kubeadmutil.ParseTemplate(CiliumDaemonSet,
		struct {
			Image                  string
			MultusImage            string
			ConfDir                string
			BinDir                 string
			SecretName             string
			ServiceAccount         string
			DaemonSetName          string
			ConfName               string
			ContainerRuntime       string
			ContainerRuntimeSocket string
			EnableBPF              string
		}{
			image,
			config.DefaultMultusImage,
			cfg.Network.Cni.ConfDir,
			cfg.Network.Cni.BinDir,
			CiliumCertSecret,
			CiliumServiceAccountName,
			daemonSetName,
			confName,
			cfg.Runtime.Engine,
			config.DefaultCriSocket[cfg.Runtime.Engine],
			enableBPF,
		})

	if err != nil {
		return fmt.Errorf("error when parsing cilium daemonset template: %v", err)
	}

	if err := createRBACRules(client, cfg.Features.PSP); err != nil {
		return fmt.Errorf("error when creating flannel RBAC rules: %v", err)
	}

	if err := createCiliumAddon(ciliumCniConfigMapBytes, ciliumEtcdConfigMapBytes, ciliumDaemonSetBytes, client); err != nil {
		return err
	}

	glog.V(1).Infof("[kubic] installed cilium CNI driver")
	return nil
}

// CreateServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func createServiceAccount(client clientset.Interface) error {
	return apiclient.CreateOrUpdateServiceAccount(client, &serviceAccount)
}

func createCiliumAddon(cniConfigMapBytes, etcdConfigMapBytes, daemonSetbytes []byte, client clientset.Interface) error {
	if cniConfigMapBytes != nil {
		ciliumCniConfigMap := &v1.ConfigMap{}
		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), cniConfigMapBytes, ciliumCniConfigMap); err != nil {
			return fmt.Errorf("unable to decode cilium configmap %v", err)
		}
		// Create the ConfigMap for cilium or update it in case it already exists
		if err := apiclient.CreateOrUpdateConfigMap(client, ciliumCniConfigMap); err != nil {
			return err
		}
	}

	ciliumEtcdConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdConfigMapBytes, ciliumEtcdConfigMap); err != nil {
		return fmt.Errorf("unable to decode cilium cni configmap %v", err)
	}

	// Create the ConfigMap for cilium or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, ciliumEtcdConfigMap); err != nil {
		return err
	}

	ciliumDaemonSet := &apps.DaemonSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), daemonSetbytes, ciliumDaemonSet); err != nil {
		return fmt.Errorf("unable to decode cilium etcd daemonset %v", err)
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
