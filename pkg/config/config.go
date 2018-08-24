package config

import (
	"fmt"
	"io/ioutil"
	"os"

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

// The kubic-init configuration
type KubicInitConfiguration struct {
	Seeder string           `json:"seeder,omitempty"`
	Cni    CniConfiguration `json:"cni,omitempty"`
}

// Load a Kubic configuration file, setting some default values
func ConfigFileAndDefaultsToKubicInitConfig(cfgPath string) (*KubicInitConfiguration, error) {
	var err error
	var internalcfg = &KubicInitConfiguration{}

	if len(cfgPath) > 0 {
		glog.V(1).Infof("[caas] loading kubic-init configuration from '%s'", cfgPath)
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
			if len(internalcfg.Seeder) == 0 {
				internalcfg.Seeder = seeder.(string)
				glog.V(2).Infof("[caas] setting seeder as %s", internalcfg.Seeder)
			}
		}
	}

	// Overwrite some values with environment variables
	if seederEnv, found := os.LookupEnv(DefaultEnvVarSeeder); found {
		internalcfg.Seeder = seederEnv
	}

	// Load the CNI configuration (or set default values)
	if len(internalcfg.Cni.Driver) == 0 {
		glog.V(3).Infof("[caas] using default CNI driver %s", DefaultCniDriver)
		internalcfg.Cni.Driver = DefaultCniDriver
	}

	// Set some networking defaults
	if len(internalcfg.Cni.PodSubnet) == 0 {
		glog.V(3).Infof("[caas] using default Pods subnet %s", DefaultPodSubnet)
		internalcfg.Cni.PodSubnet = DefaultPodSubnet
	}
	if len(internalcfg.Cni.ServiceSubnet) == 0 {
		glog.V(3).Infof("[caas] using default services subnet %s", DefaultServiceSubnet)
		internalcfg.Cni.ServiceSubnet = DefaultServiceSubnet
	}

	glog.V(8).Infof("kubic-init configuration:\n%s", spew.Sdump(internalcfg))

	return internalcfg, nil
}

// Copy some settings to a master configuration
func KubicInitConfigToMasterConfig(kubicCfg *KubicInitConfiguration, masterCfg *kubeadmapiv1alpha2.MasterConfiguration) error {
	masterCfg.Networking.PodSubnet = kubicCfg.Cni.PodSubnet
	return nil
}

// Copy some settings to a node configuration
func KubicInitConfigToNodeConfig(kubicCfg *KubicInitConfiguration, nodeCfg *kubeadmapiv1alpha2.NodeConfiguration) error {
	nodeCfg.DiscoveryTokenAPIServers = []string{kubicCfg.Seeder}
	return nil
}
