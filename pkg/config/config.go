package config

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	"github.com/yuroyoro/swalker"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1alpha2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha2"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
)

// The CNI configuration
// Subnets details are specified in the kubeadm configuration file
type CniConfiguration struct {
	Driver        string `json:"driver,omitempty"`
	Image         string `json:"image,omitempty"`
}

type ClusterFormationConfiguration struct {
	Seeder      string `json:"seeder,omitempty"`
	Token       string `json:"token,omitempty"`
	AutoApprove bool   `json:"autoApprove,omitempty"`
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
	PodSubnet     string `json:"podSubnet,omitempty"`
	ServiceSubnet string `json:"serviceSubnet,omitempty"`
}

type RuntimeConfiguration struct {
	Engine string `json:"engine,omitempty"`
}

// The kubic-init configuration
type KubicInitConfiguration struct {
	Network          NetworkConfiguration          `json:"network,omitempty"`
	ClusterFormation ClusterFormationConfiguration `json:"clusterFormation,omitempty"`
	Certificates     CertsConfiguration            `json:"certificates,omitempty"`
	Runtime          RuntimeConfiguration          `json:"runtime,omitempty"`
}

// Load a Kubic configuration file, setting some default values
func ConfigFileAndDefaultsToKubicInitConfig(cfgPath string) (*KubicInitConfiguration, error) {
	var err error
	var internalcfg = &KubicInitConfiguration{}

	// After loading the YAML file all unset values will have default values.
	// That means that all booleans will be false... but we cannot know if users
	// explictly set those "false", so we must set some defaults _before_
	// loading the YAML file
	internalcfg.ClusterFormation.AutoApprove = true

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

	// Set the default container engine
	if len(internalcfg.Runtime.Engine) == 0 {
		glog.V(3).Infof("[kubic] defaults: runtime engine %s", DefaultRuntimeEngine)
		internalcfg.Runtime.Engine = DefaultRuntimeEngine
	}

	// Load the CNI configuration (or set default values)
	if len(internalcfg.Network.Cni.Driver) == 0 {
		glog.V(3).Infof("[kubic] defaults: CNI driver '%s'", DefaultCniDriver)
		internalcfg.Network.Cni.Driver = DefaultCniDriver
	}

	// Set some networking defaults
	if len(internalcfg.Network.PodSubnet) == 0 {
		glog.V(3).Infof("[kubic] defaults: Pods subnet %s", DefaultPodSubnet)
		internalcfg.Network.PodSubnet = DefaultPodSubnet
	}
	if len(internalcfg.Network.Cni.Image) == 0 {
		glog.V(3).Infof("[kubic] defaults: CNI image '%s'", DefaultCniImage)
		internalcfg.Network.Cni.Image = DefaultCniImage
	}
	if len(internalcfg.Network.ServiceSubnet) == 0 {
		glog.V(3).Infof("[kubic] defaults: services subnet '%s'", DefaultServiceSubnet)
		internalcfg.Network.ServiceSubnet = DefaultServiceSubnet
	}

	glog.V(8).Infof("[kubic] after parsing the config file:\n%s", spew.Sdump(internalcfg))

	return internalcfg, nil
}

// ToMasterConfig copies some settings to a Master configuration
func (kubicCfg KubicInitConfiguration) ToMasterConfig(masterCfg *kubeadmapiv1alpha2.MasterConfiguration, featureGates map[string]bool) error {

	masterCfg.FeatureGates = featureGates

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

	if len(kubicCfg.Network.PodSubnet) > 0 {
		glog.V(3).Infof("[kubic] using Pods subnet '%s'", kubicCfg.Network.PodSubnet)
		masterCfg.Networking.PodSubnet = kubicCfg.Network.PodSubnet
	}

	if len(kubicCfg.Network.ServiceSubnet) > 0 {
		glog.V(3).Infof("[kubic] using services subnet '%s'", kubicCfg.Network.ServiceSubnet)
		masterCfg.Networking.ServiceSubnet = kubicCfg.Network.ServiceSubnet
	}

	if len(kubicCfg.Network.Dns.Domain) > 0 {
		glog.V(3).Infof("[kubic] using DNS domain '%s'", kubicCfg.Network.Dns.Domain)
		masterCfg.Networking.DNSDomain = kubicCfg.Network.Dns.Domain
	}

	if len(kubicCfg.Network.Dns.ExternalFqdn) > 0 {
		masterCfg.API.ControlPlaneEndpoint = kubicCfg.Network.Dns.ExternalFqdn[0]
		// TODO: add all the other ExternalFqdn's to the certs
	}

	glog.V(3).Infof("[kubic] using container engine '%s'", kubicCfg.Runtime.Engine)
	if socket, ok := DefaultCriSocket[kubicCfg.Runtime.Engine]; ok {
		glog.V(3).Infof("[kubic] setting CRI socket '%s'", socket)
		masterCfg.NodeRegistration.KubeletExtraArgs = map[string]string{}
		masterCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		masterCfg.NodeRegistration.CRISocket = socket
	}

	if glog.V(8) {
		marshalled, err := kubeadmutil.MarshalToYamlForCodecs(masterCfg, kubeadmapiv1alpha2.SchemeGroupVersion, scheme.Codecs)
		if err != nil {
			return err
		}
		glog.Infof("[kubic] master configuration produced:\n%s", marshalled)
	}

	return nil
}

// ToNodeConfig copies some settings to a Node configuration
func (kubicCfg KubicInitConfiguration) ToNodeConfig(nodeCfg *kubeadmapiv1alpha2.NodeConfiguration, featureGates map[string]bool) error {
	nodeCfg.FeatureGates = featureGates
	nodeCfg.DiscoveryTokenAPIServers = []string{kubicCfg.ClusterFormation.Seeder}
	nodeCfg.Token = kubicCfg.ClusterFormation.Token

	// Disable the ca.crt verification if no hash has been provided
	// TODO: users should be able to provide some other methods, like a ca.crt, etc
	if len(kubicCfg.Certificates.CaHash) == 0 {
		glog.V(1).Infoln("WARNING: we will not verify the identity of the seeder")
		nodeCfg.DiscoveryTokenUnsafeSkipCAVerification = true
	}

	glog.V(3).Infof("[kubic] using container engine '%s'", kubicCfg.Runtime.Engine)
	if socket, ok := DefaultCriSocket[kubicCfg.Runtime.Engine]; ok {
		glog.V(3).Infof("[kubic] setting CRI socket '%s'", socket)
		nodeCfg.NodeRegistration.KubeletExtraArgs = map[string]string{}
		nodeCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		nodeCfg.NodeRegistration.CRISocket = socket
	}

	if glog.V(8) {
		marshalled, err := kubeadmutil.MarshalToYamlForCodecs(nodeCfg, kubeadmapiv1alpha2.SchemeGroupVersion, scheme.Codecs)
		if err != nil {
			return err
		}
		glog.Infof("[kubic] node configuration produced:\n%s", marshalled)
	}

	return nil
}

// SetVars parses a list of assignments (like "key=value"), where "key"
// is a path in the configuration hierarchy (ie, "Network.Cni.Driver")
func (kubicCfg *KubicInitConfiguration) SetVars(vars []string) error {
	if len(vars) > 0 {
		var err error
		for _, v := range vars {
			components := strings.Split(v, "=")
			if len(components) != 2 {
				return fmt.Errorf("cannot parse '%s' as an assignment", v)
			}

			glog.V(8).Infof("[kubic] setting '%s'='%s'", components[0], components[1])
			err = swalker.Write(components[0], kubicCfg, components[1])
			if err != nil {
				return err
			}
		}

		glog.V(8).Infof("[kubic] after parsing variables:\n%s", spew.Sdump(kubicCfg))
	}

	return nil
}
