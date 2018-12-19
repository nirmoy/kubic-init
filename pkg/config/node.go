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

package config

import (
	"crypto/x509"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	kubicutil "github.com/kubic-project/kubic-init/pkg/util"
)

// GetNodeNameFromKubeletConfig gets the node name from a kubelet config file
func GetNodeNameFromKubeletConfig() (string, error) {
	kubeconfigDir := constants.KubernetesDir

	// loads the kubelet.conf file
	fileName := filepath.Join(kubeconfigDir, constants.KubeletKubeConfigFileName)
	config, err := clientcmd.LoadFromFile(fileName)
	if err != nil {
		return "", err
	}

	// gets the info about the current user
	authInfo := config.AuthInfos[config.Contexts[config.CurrentContext].AuthInfo]

	// gets the X509 certificate with current user credentials
	var certs []*x509.Certificate
	if len(authInfo.ClientCertificateData) > 0 {
		// if the config file uses an embedded x509 certificate (e.g. kubelet.conf created by kubeadm), parse it
		if certs, err = certutil.ParseCertsPEM(authInfo.ClientCertificateData); err != nil {
			return "", err
		}
	} else if len(authInfo.ClientCertificate) > 0 {
		// if the config file links an external x509 certificate (e.g. kubelet.conf created by TLS bootstrap), load it
		if certs, err = certutil.CertsFromFile(authInfo.ClientCertificate); err != nil {
			return "", err
		}
	} else {
		return "", errors.New("invalid kubelet.conf. X509 certificate expected")
	}

	// We are only putting one certificate in the certificate pem file, so it's safe to just pick the first one
	// TODO: Support multiple certs here in order to be able to rotate certs
	cert := certs[0]

	// gets the node name from the certificate common name
	return strings.TrimPrefix(cert.Subject.CommonName, constants.NodesUserPrefix), nil
}

// KubeadmLeftovers returns true if some kubeadm configuration files are present
func KubeadmLeftovers() bool {
	manifestsDir := filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.ManifestsSubDirName)
	mustNotExist := []string{
		kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.KubeAPIServer, manifestsDir),
		kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.KubeControllerManager, manifestsDir),
		kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.KubeScheduler, manifestsDir),
		kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.Etcd, manifestsDir),
		filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.KubeletKubeConfigFileName),
		filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.KubeletBootstrapKubeConfigFileName),
	}

	for _, f := range mustNotExist {
		if kubicutil.FileExists(f) {
			return true
		}
	}

	return false
}

// PrintNodeProperties prints some node properties
func PrintNodeProperties(node *v1.Node) {
	glog.V(1).Infoln("[kubic] node properties:")
	if len(node.Status.Conditions) > 0 {
		last := len(node.Status.Conditions) - 1
		glog.V(1).Infof("[kubic] - last condition: %s", node.Status.Conditions[last].Type)
	}
	glog.V(1).Infof("[kubic] - addresses: %s", node.Status.Addresses)
	glog.V(1).Infof("[kubic] - kubelet: %s", node.Status.NodeInfo.KubeletVersion)
	glog.V(1).Infof("[kubic] - OS: %s", node.Status.NodeInfo.OperatingSystem)
	glog.V(1).Infof("[kubic] - Distribution: %s", node.Status.NodeInfo.OSImage)
	glog.V(1).Infof("[kubic] - machine ID: %s", node.Status.NodeInfo.MachineID)
	glog.V(1).Infof("[kubic] - runtime: %s", node.Status.NodeInfo.ContainerRuntimeVersion)
	glog.V(1).Infof("[kubic] - created at: %s", node.CreationTimestamp)
	glog.V(1).Infoln("[kubic] - labels:")
	for k, v := range node.Labels {
		glog.V(1).Infof("[kubic]   - %s = %s", k, v)
	}

}
