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
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"

	"github.com/kubic-project/kubic-init/pkg/config"
)

// NewJoin starts a new Join with kubeadm
func NewJoin(kubicCfg *config.KubicInitConfiguration, args ...string) error {

	args = append(args,
		getIgnorePreflightArg(),
		getVerboseArg())

	return kubeadmWithConfig("join", kubicCfg, toJoinConfig, args...)
}

// toJoinConfig copies some settings to a Join configuration
func toJoinConfig(kubicCfg *config.KubicInitConfiguration, featureGates map[string]bool) ([]byte, error) {

	nodeCfg := &kubeadmapiv1beta1.JoinConfiguration{
		NodeRegistration: kubeadmapiv1beta1.NodeRegistrationOptions{
			KubeletExtraArgs: config.DefaultKubeletSettings,
		},
		Discovery: kubeadmapiv1beta1.Discovery{
			BootstrapToken: &kubeadmapiv1beta1.BootstrapTokenDiscovery{
				APIServerEndpoint: kubicCfg.ClusterFormation.Seeder,
				Token:             kubicCfg.ClusterFormation.Token,
			},
		},
	}

	// Disable the ca.crt verification if no hash has been provided
	// TODO: users should be able to provide some other methods, like a ca.crt, etc
	if len(kubicCfg.Certificates.CaHash) == 0 {
		glog.V(1).Infoln("WARNING: we will not verify the identity of the seeder")
		nodeCfg.Discovery.BootstrapToken.UnsafeSkipCAVerification = true
	}

	glog.V(3).Infof("[kubic] using container engine '%s'", kubicCfg.Runtime.Engine)
	if socket, ok := config.DefaultCriSocket[kubicCfg.Runtime.Engine]; ok {
		glog.V(3).Infof("[kubic] setting CRI socket '%s'", socket)
		nodeCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		nodeCfg.NodeRegistration.CRISocket = socket
	}

	kubeadmscheme.Scheme.Default(nodeCfg)
	nodebytes, err := kubeadmutil.MarshalToYamlForCodecs(nodeCfg, kubeadmapiv1beta1.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return []byte{}, err
	}
	return nodebytes, nil
}
