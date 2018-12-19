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
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	cmdutil "k8s.io/kubernetes/cmd/kubeadm/app/cmd/util"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	certsphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"
	kubeconfigphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	"github.com/kubic-project/kubic-init/pkg/config"
)

// initData defines all the runtime information used when running the kubeadm init worklow;
// this data is shared across all the phases that are included in the workflow.
type initData struct {
	cfg                   *kubeadmapi.InitConfiguration
	skipTokenPrint        bool
	dryRun                bool
	kubeconfigDir         string
	kubeconfigPath        string
	ignorePreflightErrors sets.String
	certificatesDir       string
	dryRunDir             string
	externalCA            bool
	client                clientset.Interface
	waiter                apiclient.Waiter
	outputWriter          io.Writer
}

// newInitData returns a new initData struct to be used for the execution of the kubeadm init workflow.
// This func takes care of validating initCfg passed to the command, and then it converts
// options into the internal InitConfiguration type that is used as input all the phases in the kubeadm init workflow
func newInitData(initCfg *kubeadmapiv1beta1.InitConfiguration) (initData, error) {

	// Either use the config file if specified, or convert public kubeadm API to the internal InitConfiguration
	// and validates InitConfiguration
	cfg, err := configutil.ConfigFileAndDefaultsToInternalConfig("", initCfg)
	if err != nil {
		return initData{}, err
	}

	ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(config.DefaultIgnoredPreflightErrors)
	kubeadmutil.CheckErr(err)

	externalCA, _ := certsphase.UsingExternalCA(cfg)
	if externalCA {
		kubeconfigDir := kubeadmconstants.KubernetesDir
		if err := kubeconfigphase.ValidateKubeconfigsForExternalCA(kubeconfigDir, cfg); err != nil {
			return initData{}, err
		}
	}

	cfg.DNS.Type = kubeadmapi.CoreDNS
	return initData{
		cfg:                   cfg,
		certificatesDir:       cfg.CertificatesDir,
		skipTokenPrint:        false,
		dryRun:                false,
		dryRunDir:             "",
		kubeconfigDir:         kubeadmconstants.KubernetesDir,
		kubeconfigPath:        kubeadmconstants.GetAdminKubeConfigPath(),
		ignorePreflightErrors: ignorePreflightErrorsSet,
		externalCA:            externalCA,
		outputWriter:          os.Stdout,
	}, nil
}

// Cfg returns initConfiguration.
func (d initData) Cfg() *kubeadmapi.InitConfiguration {
	return d.cfg
}

// DryRun returns the DryRun flag.
func (d initData) DryRun() bool {
	return d.dryRun
}

// SkipTokenPrint returns the SkipTokenPrint flag.
func (d initData) SkipTokenPrint() bool {
	return d.skipTokenPrint
}

// IgnorePreflightErrors returns the IgnorePreflightErrors flag.
func (d initData) IgnorePreflightErrors() sets.String {
	return d.ignorePreflightErrors
}

// CertificateWriteDir returns the path to the certificate folder or the temporary folder path in case of DryRun.
func (d initData) CertificateWriteDir() string {
	return d.certificatesDir
}

// CertificateDir returns the CertificateDir as originally specified by the user.
func (d initData) CertificateDir() string {
	return d.certificatesDir
}

// KubeConfigDir returns the path of the Kubernetes configuration folder or the temporary folder path in case of DryRun.
func (d initData) KubeConfigDir() string {
	return d.kubeconfigDir
}

// KubeConfigPath returns the path to the kubeconfig file to use for connecting to Kubernetes
func (d initData) KubeConfigPath() string {
	return d.kubeconfigPath
}

// ManifestDir returns the path where manifest should be stored or the temporary folder path in case of DryRun.
func (d initData) ManifestDir() string {
	return kubeadmconstants.GetStaticPodDirectory()
}

// KubeletDir returns path of the kubelet configuration folder or the temporary folder in case of DryRun.
func (d initData) KubeletDir() string {
	return kubeadmconstants.KubeletRunDirectory
}

// ExternalCA returns true if an external CA is provided by the user.
func (d initData) ExternalCA() bool {
	return d.externalCA
}

// OutputWriter returns the io.Writer used to write output to by this command.
func (d initData) OutputWriter() io.Writer {
	return d.outputWriter
}

// Client returns a Kubernetes client to be used by kubeadm.
// This function is implemented as a singleton, thus avoiding to recreate the client when it is used by different phases.
// Important. This function must be called after the admin.conf kubeconfig file is created.
func (d initData) Client() (clientset.Interface, error) {
	if d.client == nil {
		// If we're acting for real, we should create a connection to the API server and wait for it to come up
		var err error
		d.client, err = kubeconfigutil.ClientSetFromFile(d.KubeConfigPath())
		if err != nil {
			return nil, err
		}
	}
	return d.client, nil
}

// Tokens returns an array of token strings.
func (d initData) Tokens() []string {
	tokens := []string{}
	for _, bt := range d.cfg.BootstrapTokens {
		tokens = append(tokens, bt.Token.String())
	}
	return tokens
}

func printJoinCommand(adminKubeConfigPath, token string, skipTokenPrint bool) error {
	joinCommand, err := cmdutil.GetJoinCommand(adminKubeConfigPath, token, skipTokenPrint)
	if err != nil {
		return err
	}

	glog.V(3).Infof("[kubic] Join command: %v\n", joinCommand)
	return nil
}

// showJoinCommand prints the join command after all the phases in init have finished
func showJoinCommand(i *initData) error {
	adminKubeConfigPath := i.KubeConfigPath()

	// Prints the join command, multiple times in case the user has multiple tokens
	for _, token := range i.Tokens() {
		if err := printJoinCommand(adminKubeConfigPath, token, i.skipTokenPrint); err != nil {
			return fmt.Errorf("failed to print join command %v", err)
		}
	}

	return nil
}

// NewCmdInit returns "kubeadm init" command.
func NewInit(kubicCfg *config.KubicInitConfiguration) error {
	featureGates, err := features.NewFeatureGate(&features.InitFeatureGates, config.DefaultFeatureGates)
	initRunner := workflow.NewRunner()
	initRunner.AppendPhase(phases.NewPreflightMasterPhase())
	initRunner.AppendPhase(phases.NewKubeletStartPhase())
	initRunner.AppendPhase(phases.NewCertsPhase())
	initRunner.AppendPhase(phases.NewKubeConfigPhase())
	initRunner.AppendPhase(phases.NewControlPlanePhase())
	initRunner.AppendPhase(phases.NewEtcdPhase())
	initRunner.AppendPhase(phases.NewWaitControlPlanePhase())
	initRunner.AppendPhase(phases.NewUploadConfigPhase())
	initRunner.AppendPhase(phases.NewMarkControlPlanePhase())
	initRunner.AppendPhase(phases.NewBootstrapTokenPhase())
	initRunner.AppendPhase(phases.NewAddonPhase())

	externalfeatures, err := toInitConfig(kubicCfg, featureGates)
	initRunner.SetDataInitializer(func(cmd *cobra.Command) (workflow.RunData, error) {
		return newInitData(externalfeatures)
	})

	c, err := initRunner.InitData()
	kubeadmutil.CheckErr(err)
	data := c.(initData)
	glog.V(3).Infof("[kubic] Using Kubernetes version: %s\n", data.cfg.KubernetesVersion)

	err = initRunner.Run()
	kubeadmutil.CheckErr(err)

	return showJoinCommand(&data)

}

// toInitConfig copies some settings to a Init configuration
func toInitConfig(kubicCfg *config.KubicInitConfiguration, featureGates map[string]bool) (*kubeadmapiv1beta1.InitConfiguration, error) {
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
	return initCfg, nil

}
