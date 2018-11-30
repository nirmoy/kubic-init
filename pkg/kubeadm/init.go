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

package kubeadm

import (
	"bytes"
	"fmt"

	"github.com/golang/glog"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"

	"github.com/kubic-project/kubic-init/pkg/config"
)

// NewInit starts a new Init with kubeadm
func NewInit(kubicCfg *config.KubicInitConfiguration, args ...string) error {

	args = append(args,
		getIgnorePreflightArg(),
		getVerboseArg())

	return kubeadmCmd("init", kubicCfg, toInitConfig, args...)
}

// toInitConfig copies some settings to a Init configuration
func toInitConfig(kubicCfg *config.KubicInitConfiguration, featureGates map[string]bool) ([]byte, error) {
	glog.V(3).Infof("[kubic] creating initialization configuration...")

	initCfg := &kubeadmapiv1beta1.InitConfiguration{
		ClusterConfiguration: kubeadmapiv1beta1.ClusterConfiguration{
			ControlPlaneEndpoint: kubicCfg.Network.DNS.ExternalFqdn,
			FeatureGates:         featureGates,
			APIServer: kubeadmapiv1beta1.APIServer{
				CertSANs: []string{},
			},
			KubernetesVersion: config.DefaultKubernetesVersion,
			Networking: kubeadmapiv1beta1.Networking{
				PodSubnet:     kubicCfg.Network.PodSubnet,
				ServiceSubnet: kubicCfg.Network.ServiceSubnet,
			},
		},
		NodeRegistration: kubeadmapiv1beta1.NodeRegistrationOptions{
			KubeletExtraArgs: config.DefaultKubeletSettings,
		},
	}

	nonEmpty := func(a, b string) string {
		if len(a) > 0 {
			return a
		}
		return b
	}

	if kubicCfg.Etcd.LocalEtcd != nil {
		initCfg.ClusterConfiguration.Etcd = kubeadmapiv1beta1.Etcd{
			Local: &kubeadmapiv1beta1.LocalEtcd{
				ImageMeta: kubeadmapiv1beta1.ImageMeta{
					ImageRepository: config.DefaultEtdcImageRepo,
					ImageTag:        config.DefaultEtdcImageTag,
				},
			},
		}
	}

	// Add some extra flags in the API server for OIDC (necessary for using Dex)
	initCfg.ClusterConfiguration.APIServer.ExtraArgs = map[string]string{
		"oidc-client-id":      nonEmpty(kubicCfg.Auth.OIDC.ClientID, config.DefaultOIDCClientID),
		"oidc-ca-file":        nonEmpty(kubicCfg.Auth.OIDC.CA, config.DefaultCertCA),
		"oidc-username-claim": nonEmpty(kubicCfg.Auth.OIDC.Username, config.DefaultOIDCUsernameClaim),
		"oidc-groups-claim":   nonEmpty(kubicCfg.Auth.OIDC.Groups, config.DefaultOIDCGroupsClaim),
	}

	if len(kubicCfg.Auth.OIDC.Issuer) > 0 {
		initCfg.ClusterConfiguration.APIServer.ExtraArgs["oidc-issuer-url"] = kubicCfg.Auth.OIDC.Issuer
	} else {
		public, err := kubicCfg.GetPublicAPIAddress()
		if err != nil {
			return nil, err
		}
		initCfg.ClusterConfiguration.APIServer.ExtraArgs["oidc-issuer-url"] = fmt.Sprintf("https://%s:%d", public, config.DefaultDexIssuerPort)
	}

	if len(kubicCfg.Network.Bind.Address) > 0 && kubicCfg.Network.Bind.Address != "127.0.0.1" {
		glog.V(8).Infof("[kubic] setting bind address: %s", kubicCfg.Network.Bind.Address)
		initCfg.LocalAPIEndpoint.AdvertiseAddress = kubicCfg.Network.Bind.Address
		initCfg.ClusterConfiguration.APIServer.CertSANs = append(initCfg.ClusterConfiguration.APIServer.CertSANs, kubicCfg.Network.Bind.Address)
	}

	// TODO: enable these two args once we have OpenSUSE images in registry.opensuse.org for k8s
	//
	// ImageRepository what container registry to pull control plane images from
	// initCfg.ImageRepository = "registry.opensuse.org"
	//
	// UnifiedControlPlaneImage specifies if a specific container image should
	// be used for all control plane components.
	// initCfg.UnifiedControlPlaneImage = ""

	if len(kubicCfg.ClusterFormation.Token) > 0 {
		glog.V(8).Infof("[kubic] adding a bootstrap token: %s", kubicCfg.ClusterFormation.Token)
		var err error
		bto := kubeadmapiv1beta1.BootstrapToken{}
		bto.Token, err = kubeadmapiv1beta1.NewBootstrapTokenString(kubicCfg.ClusterFormation.Token)
		if err != nil {
			return nil, err
		}
		initCfg.BootstrapTokens = []kubeadmapiv1beta1.BootstrapToken{bto}
	}

	if len(kubicCfg.Network.DNS.Domain) > 0 {
		glog.V(3).Infof("[kubic] using DNS domain '%s'", kubicCfg.Network.DNS.Domain)
		initCfg.Networking.DNSDomain = kubicCfg.Network.DNS.Domain
	}

	if len(kubicCfg.Network.DNS.ExternalFqdn) > 0 {
		// TODO: add all the other ExternalFqdn's to the certs
		initCfg.APIServer.CertSANs = append(initCfg.APIServer.CertSANs, kubicCfg.Network.DNS.ExternalFqdn)
	}

	glog.V(3).Infof("[kubic] using container engine '%s'", kubicCfg.Runtime.Engine)
	if socket, ok := config.DefaultCriSocket[kubicCfg.Runtime.Engine]; ok {
		glog.V(3).Infof("[kubic] setting CRI socket '%s'", socket)
		initCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		initCfg.NodeRegistration.CRISocket = socket
	}

	kubeadmscheme.Scheme.Default(initCfg)

	initbytes, err := kubeadmutil.MarshalToYamlForCodecs(initCfg, kubeadmapiv1beta1.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return []byte{}, err
	}
	allFiles := [][]byte{initbytes}

	clusterbytes, err := kubeadmutil.MarshalToYamlForCodecs(&initCfg.ClusterConfiguration, kubeadmapiv1beta1.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return []byte{}, err
	}
	allFiles = append(allFiles, clusterbytes)

	return bytes.Join(allFiles, []byte(kubeadmconstants.YAMLDocumentSeparator)), nil
}
