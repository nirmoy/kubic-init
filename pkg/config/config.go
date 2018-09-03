package config

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	kubeadmapiv1alpha2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha2"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
)

// The CNI configuration
// Subnets details are specified in the kubeadm configuration file
type CniConfiguration struct {
	Driver        string `json:"driver,omitempty"`
	PodSubnet     string `json:"podSubnet,omitempty"`
	ServiceSubnet string `json:"serviceSubnet,omitempty"`
}

type ClusterFormationConfiguration struct {
	Seeder string `json:"seeder,omitempty"`
	Token  string `json:"token,omitempty"`
}

type CertsConfiguration struct {
	// TODO
	CaHash string `json:"caCrtHash,omitempty"`
}

type DNSConfiguration struct {
	Domain       string   `json:"domain,omitempty"`
	ExternalFqdn []string `json:"externalFqdn,omitempty"`
}

type NetworkConfiguration struct {
	Cni CniConfiguration `json:"cni,omitempty"`
	Dns DNSConfiguration `json:"dns,omitempty"`
}

// The kubic-init configuration
type KubicInitConfiguration struct {
	Network          NetworkConfiguration          `json:"network,omitempty"`
	ClusterFormation ClusterFormationConfiguration `json:"clusterFormation,omitempty"`
	Certificates     CertsConfiguration            `json:"certificates,omitempty"`
}

// Load a Kubic configuration file, setting some default values
func ConfigFileAndDefaultsToKubicInitConfig(cfgPath string) (*KubicInitConfiguration, error) {
	var err error
	var internalcfg = &KubicInitConfiguration{}

	if len(cfgPath) > 0 {
		glog.V(1).Infof("[kubic] loading kubic-init configuration from '%s'", cfgPath)
		if os.Stat(cfgPath); err != nil {
			return nil, fmt.Errorf("%q does not exist: %v", cfgPath, err)
		}

		b, err := ioutil.ReadFile(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read config from %q [%v]", cfgPath, err)
		}

		decoded, err := kubeadmutil.LoadYAML(b)
		if err != nil {
			return nil, fmt.Errorf("unable to decode config from bytes: %v", err)
		}

		// TODO: check the decoded['kind']

		seeder := decoded["seeder"]
		if seeder != nil && len(seeder.(string)) > 0 {
			if len(internalcfg.ClusterFormation.Seeder) == 0 {
				internalcfg.ClusterFormation.Seeder = seeder.(string)
				glog.V(2).Infof("[kubic] setting seeder as %s", internalcfg.ClusterFormation.Seeder)
			}
		}
	}

	// Overwrite some values with environment variables
	if seederEnv, found := os.LookupEnv(DefaultEnvVarSeeder); found {
		glog.V(3).Infof("[kubic] setting cluster seeder %s", seederEnv)
		internalcfg.ClusterFormation.Seeder = seederEnv
	}
	if tokenEnv, found := os.LookupEnv(DefaultEnvVarToken); found {
		glog.V(3).Infof("[kubic] setting cluster token '%s'", tokenEnv)
		internalcfg.ClusterFormation.Token = tokenEnv
	}

	// The seeder is a IP:PORT, so parse the current seeder and reformat it appropriately
	if len(internalcfg.ClusterFormation.Seeder) > 0 {
		seeder := internalcfg.ClusterFormation.Seeder
		if !strings.HasPrefix(seeder, "http") {
			seeder = fmt.Sprintf("https://%s", internalcfg.ClusterFormation.Seeder)
		}
		u, err := url.Parse(seeder)
		if err != nil {
			return nil, err
		}
		port := u.Port()

		// if no port has been provided, use the API server default port
		if len(port) == 0 {
			port = fmt.Sprintf("%d", DefaultAPIServerPort)
		}

		internalcfg.ClusterFormation.Seeder = fmt.Sprintf("%s:%s", u.Hostname(), port)
	}

	// Load the CNI configuration (or set default values)
	if len(internalcfg.Network.Cni.Driver) == 0 {
		glog.V(3).Infof("[kubic] using default CNI driver %s", DefaultCniDriver)
		internalcfg.Network.Cni.Driver = DefaultCniDriver
	}

	// Set some networking defaults
	if len(internalcfg.Network.Cni.PodSubnet) == 0 {
		glog.V(3).Infof("[kubic] using default Pods subnet %s", DefaultPodSubnet)
		internalcfg.Network.Cni.PodSubnet = DefaultPodSubnet
	}
	if len(internalcfg.Network.Cni.ServiceSubnet) == 0 {
		glog.V(3).Infof("[kubic] using default services subnet %s", DefaultServiceSubnet)
		internalcfg.Network.Cni.ServiceSubnet = DefaultServiceSubnet
	}

	glog.V(8).Infof("kubic-init configuration:\n%s", spew.Sdump(internalcfg))

	return internalcfg, nil
}

// Copy some settings to a master configuration
func KubicInitConfigToMasterConfig(kubicCfg *KubicInitConfiguration, masterCfg *kubeadmapiv1alpha2.MasterConfiguration) error {
	masterCfg.Networking.PodSubnet = kubicCfg.Network.Cni.PodSubnet

	if len(kubicCfg.ClusterFormation.Token) > 0 {
		glog.V(8).Infof("[kubic] adding a default bootstrap token: %s", kubicCfg.ClusterFormation.Token)
		var err error
		bto := kubeadmapiv1alpha2.BootstrapToken{}
		kubeadmapiv1alpha2.SetDefaults_BootstrapToken(&bto)
		bto.Token, err = kubeadmapiv1alpha2.NewBootstrapTokenString(kubicCfg.ClusterFormation.Token)
		if err != nil {
			return err
		}

		masterCfg.BootstrapTokens = []kubeadmapiv1alpha2.BootstrapToken{bto}
	}

	return nil
}

// Copy some settings to a node configuration
func KubicInitConfigToNodeConfig(kubicCfg *KubicInitConfiguration, nodeCfg *kubeadmapiv1alpha2.NodeConfiguration) error {
	nodeCfg.DiscoveryTokenAPIServers = []string{kubicCfg.ClusterFormation.Seeder}
	nodeCfg.Token = kubicCfg.ClusterFormation.Token

	// Disable the ca.crt verification if no hash has been provided
	// TODO: users should be able to provide some other methods, like a ca.crt, etc
	if len(kubicCfg.Certificates.CaHash) == 0 {
		glog.V(1).Infoln("WARNING: we will not verify the identity of the seeder")
		nodeCfg.DiscoveryTokenUnsafeSkipCAVerification = true
	}

	return nil
}
