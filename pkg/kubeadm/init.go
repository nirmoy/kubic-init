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
	"fmt"

	"github.com/golang/glog"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"

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

	initCfg := &kubeadmapi.InitConfiguration{
		ClusterConfiguration: kubeadmapi.ClusterConfiguration{
			ControlPlaneEndpoint: kubicCfg.Network.Dns.ExternalFqdn,
			FeatureGates:         featureGates,
			APIServerCertSANs:    []string{},
			KubernetesVersion:    config.DefaultKubernetesVersion,
			Networking: kubeadmapi.Networking{
				PodSubnet:     kubicCfg.Network.PodSubnet,
				ServiceSubnet: kubicCfg.Network.ServiceSubnet,
			},
		},
		NodeRegistration: kubeadmapi.NodeRegistrationOptions{
			KubeletExtraArgs: config.DefaultKubeletSettings,
		},
	}

	nonEmpty := func(a, b string) string {
		if len(a) > 0 {
			return a
		}
		return b
	}

	// Add some extra flags in the API server for OIDC (necessary for using Dex)
	initCfg.ClusterConfiguration.APIServerExtraArgs = map[string]string{
		"oidc-client-id":      nonEmpty(kubicCfg.Auth.OIDC.ClientID, config.DefaultOIDCClientID),
		"oidc-ca-file":        nonEmpty(kubicCfg.Auth.OIDC.CA, config.DefaultCertCA),
		"oidc-username-claim": nonEmpty(kubicCfg.Auth.OIDC.Username, config.DefaultOIDCUsernameClaim),
		"oidc-groups-claim":   nonEmpty(kubicCfg.Auth.OIDC.Groups, config.DefaultOIDCGroupsClaim),
	}

	if len(kubicCfg.Auth.OIDC.Issuer) > 0 {
		initCfg.ClusterConfiguration.APIServerExtraArgs["oidc-issuer-url"] = kubicCfg.Auth.OIDC.Issuer
	} else {
		public, err := kubicCfg.GetPublicAPIAddress()
		if err != nil {
			return nil, err
		}
		initCfg.ClusterConfiguration.APIServerExtraArgs["oidc-issuer-url"] = fmt.Sprintf("https://%s:%d", public, config.DefaultDexIssuerPort)
	}

	if len(kubicCfg.Network.Bind.Address) > 0 && kubicCfg.Network.Bind.Address != "127.0.0.1" {
		glog.V(8).Infof("[kubic] setting bind address: %s", kubicCfg.Network.Bind.Address)
		initCfg.APIEndpoint.AdvertiseAddress = kubicCfg.Network.Bind.Address
		initCfg.ClusterConfiguration.APIServerCertSANs = append(initCfg.ClusterConfiguration.APIServerCertSANs, kubicCfg.Network.Bind.Address)
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
		bto := kubeadmapi.BootstrapToken{}
		bto.Token, err = kubeadmapi.NewBootstrapTokenString(kubicCfg.ClusterFormation.Token)
		if err != nil {
			return nil, err
		}
		initCfg.BootstrapTokens = []kubeadmapi.BootstrapToken{bto}
	}

	if len(kubicCfg.Network.Dns.Domain) > 0 {
		glog.V(3).Infof("[kubic] using DNS domain '%s'", kubicCfg.Network.Dns.Domain)
		initCfg.Networking.DNSDomain = kubicCfg.Network.Dns.Domain
	}

	if len(kubicCfg.Network.Dns.ExternalFqdn) > 0 {
		// TODO: add all the other ExternalFqdn's to the certs
		initCfg.APIServerCertSANs = append(initCfg.APIServerCertSANs, kubicCfg.Network.Dns.ExternalFqdn)
	}

	glog.V(3).Infof("[kubic] using container engine '%s'", kubicCfg.Runtime.Engine)
	if socket, ok := config.DefaultCriSocket[kubicCfg.Runtime.Engine]; ok {
		glog.V(3).Infof("[kubic] setting CRI socket '%s'", socket)
		initCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		initCfg.NodeRegistration.CRISocket = socket
	}

	kubeadmscheme.Scheme.Default(initCfg)
	return configutil.MarshalKubeadmConfigObject(initCfg)
}
