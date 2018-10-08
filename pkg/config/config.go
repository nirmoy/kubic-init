/*
Copyright 2018 SUSE LINUX GmbH, Nuernberg, Germany..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:generate go run ../../vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go -O zz_generated.deepcopy -i ./... -h ../../hack/boilerplate.go.txt

package config

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes/kubernetes/cmd/kubeadm/app/util/apiclient"
	"github.com/yuroyoro/swalker"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1alpha2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha2"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"

	kubicutil "github.com/kubic-project/kubic-init/pkg/util"
)

// The CNI configuration
// Subnets details are specified in the kubeadm configuration file
type CniConfiguration struct {
	Driver string `json:"driver,omitempty"`
	Image  string `json:"image,omitempty"`
}

type ClusterFormationConfiguration struct {
	Seeder      string `json:"seeder,omitempty"`
	Token       string `json:"token,omitempty"`
	AutoApprove bool   `json:"autoApprove,omitempty"`
}

type CertsConfiguration struct {
	// TODO
	Directory string `yaml:"directory,omitempty"`
	CaHash    string `yaml:"caCrtHash,omitempty"`
}

type DNSConfiguration struct {
	Domain       string `yaml:"domain,omitempty"`
	ExternalFqdn string `yaml:"externalFqdn,omitempty"`
}

type ProxyConfiguration struct {
	Http       string `yaml:"http,omitempty"`
	Https      string `yaml:"https,omitempty"`
	NoProxy    string `yaml:"noProxy,omitempty"`
	SystemWide bool   `yaml:"systemWide,omitempty"`
}

type BindConfiguration struct {
	Address   string `yaml:"address,omitempty"`
	Interface string `yaml:"interface,omitempty"`
}

type NetworkConfiguration struct {
	Bind          BindConfiguration  `yaml:"bind,omitempty"`
	Cni           CniConfiguration   `yaml:"cni,omitempty"`
	Dns           DNSConfiguration   `yaml:"dns,omitempty"`
	Proxy         ProxyConfiguration `yaml:"proxy,omitempty"`
	PodSubnet     string             `yaml:"podSubnet,omitempty"`
	ServiceSubnet string             `yaml:"serviceSubnet,omitempty"`
}

type ManagerConfiguration struct {
	Image string `yaml:"image,omitempty"`
}

type RuntimeConfiguration struct {
	Engine string `yaml:"engine,omitempty"`
}

type FeaturesConfiguration struct {
	PSP bool `yaml:"PSP,omitempty"`
}

type DexLDAPUserConfiguration struct {
	BaseDN  string            `yaml:"baseDN,omitempty"`
	Filter  string            `yaml:"filter,omitempty"`
	AttrMap map[string]string `yaml:"attrMap,omitempty"`
}

type ServicesConfiguration struct {
}

// The kubic-init configuration
//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
type KubicInitConfiguration struct {
	metav1.TypeMeta
	Network          NetworkConfiguration          `yaml:"network,omitempty"`
	Manager          ManagerConfiguration          `yaml:"manager,omitempty"`
	ClusterFormation ClusterFormationConfiguration `yaml:"clusterFormation,omitempty"`
	Certificates     CertsConfiguration            `yaml:"certificates,omitempty"`
	Runtime          RuntimeConfiguration          `yaml:"runtime,omitempty"`
	Features         FeaturesConfiguration         `yaml:"features,omitempty"`
	Services         ServicesConfiguration         `yaml:"services,omitempty"`
}

// Load a Kubic configuration file, setting some default values
func ConfigFileAndDefaultsToKubicInitConfig(cfgPath string) (*KubicInitConfiguration, error) {
	var err error
	var internalcfg = KubicInitConfiguration{}

	// set some defaults
	internalcfg.Certificates.Directory = DefaultCertsDirectory
	internalcfg.Manager.Image = DefaultKubicInitImage

	// After loading the YAML file all unset values will have default values.
	// That means that all booleans will be false... but we cannot know if users
	// explictly set those "false", so we must set some defaults _before_
	// loading the YAML file
	internalcfg.ClusterFormation.AutoApprove = true
	internalcfg.Features.PSP = false

	if len(cfgPath) > 0 {
		glog.V(1).Infof("[kubic] loading kubic-init configuration from '%s'", cfgPath)
		if os.Stat(cfgPath); err != nil {
			return nil, fmt.Errorf("%q does not exist: %v", cfgPath, err)
		}

		b, err := ioutil.ReadFile(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read config from %q [%v]", cfgPath, err)
		}

		if err = yaml.Unmarshal(b, &internalcfg); err != nil {
			return nil, fmt.Errorf("unable to decode config from bytes: %v", err)
		}

		// TODO: check the internalcfg['kind']
	}

	// Overwrite some values with environment variables
	if seederEnv, found := os.LookupEnv(DefaultEnvVarSeeder); found {
		glog.V(3).Infof("[kubic] environment: setting cluster seeder %s", seederEnv)
		internalcfg.ClusterFormation.Seeder = seederEnv
	}

	if tokenEnv, found := os.LookupEnv(DefaultEnvVarToken); found {
		glog.V(3).Infof("[kubic] environment: setting cluster token '%s'", tokenEnv)
		internalcfg.ClusterFormation.Token = tokenEnv
	}

	if managerEnv, found := os.LookupEnv(DefaultEnvVarManager); found {
		glog.V(3).Infof("[kubic] environment: setting kubic-manager image '%s'", managerEnv)
		internalcfg.Manager.Image = managerEnv
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
	if len(internalcfg.Network.Dns.Domain) == 0 {
		glog.V(3).Infof("[kubic] defaults: DNS domain '%s'", DefaultDNSDomain)
		internalcfg.Network.Dns.Domain = DefaultDNSDomain
	}

	if glog.V(8) {
		marshalled, err := yaml.Marshal(internalcfg)
		if err != nil {
			return nil, err
		}
		glog.Infof("[kubic] after parsing the config file:\n%s", marshalled)
	}

	return &internalcfg, nil
}

// ToMasterConfig copies some settings to a Master configuration
func (kubicCfg KubicInitConfiguration) ToMasterConfig(featureGates map[string]bool) (*kubeadmapiv1alpha2.MasterConfiguration, error) {
	masterCfg := &kubeadmapiv1alpha2.MasterConfiguration{}

	masterCfg.FeatureGates = featureGates
	masterCfg.NodeRegistration.KubeletExtraArgs = DefaultKubeletSettings

	if len(kubicCfg.ClusterFormation.Token) > 0 {
		glog.V(8).Infof("[kubic] adding a default bootstrap token: %s", kubicCfg.ClusterFormation.Token)
		var err error
		bto := kubeadmapiv1alpha2.BootstrapToken{}
		kubeadmapiv1alpha2.SetDefaults_BootstrapToken(&bto)
		bto.Token, err = kubeadmapiv1alpha2.NewBootstrapTokenString(kubicCfg.ClusterFormation.Token)
		if err != nil {
			return nil, err
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
		if masterCfg.KubeletConfiguration.BaseConfig != nil {
			// TODO: should we create this "BaseConfig" for the kubelet?
			masterCfg.KubeletConfiguration.BaseConfig.ClusterDomain = kubicCfg.Network.Dns.Domain
		}
	}

	if len(kubicCfg.Network.Dns.ExternalFqdn) > 0 {
		masterCfg.API.ControlPlaneEndpoint = kubicCfg.Network.Dns.ExternalFqdn
		// TODO: add all the other ExternalFqdn's to the certs

		masterCfg.APIServerCertSANs = append(masterCfg.APIServerCertSANs, kubicCfg.Network.Dns.ExternalFqdn)
	}

	glog.V(3).Infof("[kubic] using container engine '%s'", kubicCfg.Runtime.Engine)
	if socket, ok := DefaultCriSocket[kubicCfg.Runtime.Engine]; ok {
		glog.V(3).Infof("[kubic] setting CRI socket '%s'", socket)
		masterCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		masterCfg.NodeRegistration.CRISocket = socket
	}

	kubeadmscheme.Scheme.Default(masterCfg)

	if glog.V(8) {
		marshalled, err := kubeadmutil.MarshalToYamlForCodecs(masterCfg, kubeadmapiv1alpha2.SchemeGroupVersion, scheme.Codecs)
		if err != nil {
			return nil, err
		}
		glog.Infof("[kubic] master configuration produced:\n%s", marshalled)
	}

	return masterCfg, nil
}

// ToNodeConfig copies some settings to a Node configuration
func (kubicCfg KubicInitConfiguration) ToNodeConfig(featureGates map[string]bool) (*kubeadmapiv1alpha2.NodeConfiguration, error) {
	nodeCfg := &kubeadmapiv1alpha2.NodeConfiguration{}

	nodeCfg.FeatureGates = featureGates
	nodeCfg.DiscoveryTokenAPIServers = []string{kubicCfg.ClusterFormation.Seeder}
	nodeCfg.NodeRegistration.KubeletExtraArgs = DefaultKubeletSettings
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
		nodeCfg.NodeRegistration.KubeletExtraArgs["container-runtime-endpoint"] = fmt.Sprintf("unix://%s", socket)
		nodeCfg.NodeRegistration.CRISocket = socket
	}

	kubeadmscheme.Scheme.Default(nodeCfg)

	if glog.V(8) {
		marshalled, err := kubeadmutil.MarshalToYamlForCodecs(nodeCfg, kubeadmapiv1alpha2.SchemeGroupVersion, scheme.Codecs)
		if err != nil {
			return nil, err
		}
		glog.Infof("[kubic] node configuration produced:\n%s", marshalled)
	}

	return nodeCfg, nil
}

// ToConfigMap uploads the configuration to a "kubic-init.yaml" file in a ConfigMap
func (kubicCfg *KubicInitConfiguration) ToConfigMap(client clientset.Interface, name string, extraLabels map[string]string) error {
	filename := filepath.Base(DefaultKubicInitConfig)

	glog.V(3).Infof("[kubic] uploading to ConfigMap %s/%s the '%s' configuration",
		metav1.NamespaceSystem, name, filename)

	// TODO: check there is no sensible information in the kubicCfg and remove it...

	marshalled, err := yaml.Marshal(kubicCfg)
	if err != nil {
		return err
	}

	if err := apiclient.CreateOrUpdateConfigMap(client, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceSystem,
			Labels:    extraLabels,
		},
		Data: map[string]string{
			filename: string(marshalled),
		},
	}); err != nil {
		return err
	}

	glog.V(3).Infof("[kubic] configuration uploaded to ConfigMap %s/%s",
		metav1.NamespaceSystem, name)

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

		if glog.V(8) {
			marshalled, err := yaml.Marshal(kubicCfg)
			if err != nil {
				return err
			}
			glog.Infof("[kubic] after parsing the list of variables:\n%s", marshalled)
		}
	}

	return nil
}

func (kubicCfg KubicInitConfiguration) IsSeeder() bool {
	return len(kubicCfg.ClusterFormation.Seeder) == 0
}

// GetBindIP gets a valid IP address where we can bind
func (kubicCfg KubicInitConfiguration) GetBindIP() (net.IP, error) {
	if len(kubicCfg.Network.Bind.Interface) > 0 {
		// TODO: not implemented yet: get the IP address for that interface
		return net.IP{}, nil
	} else {
		defaultAddrStr := "0.0.0.0"
		if len(kubicCfg.Network.Bind.Address) > 0 {
			defaultAddrStr = kubicCfg.Network.Bind.Address
		}

		defaultAddr := net.ParseIP(defaultAddrStr)
		bindIP, err := utilnet.ChooseBindAddress(defaultAddr)
		if err != nil {
			return nil, err
		}
		return bindIP, nil
	}
}

// GetPublicAPIAddress gets a DNS name (or IP address)
// that can be used for reaching the API server
func (kubicCfg KubicInitConfiguration) GetPublicAPIAddress() (string, error) {
	if len(kubicCfg.Network.Dns.ExternalFqdn) > 0 {
		return kubicCfg.Network.Dns.ExternalFqdn, nil
	} else {
		// ok, we don't have a user-provided DNS name, so we
		// must apply some heuristics...

		// 1. if this is the seeder, there will be an API server running here:
		// just return the local IP address as the IP address of the API server
		if kubicCfg.IsSeeder() {
			localIP, err := utilnet.ChooseHostInterface()
			if err != nil {
				return "", err
			}
			return localIP.String(), nil
		}
	}
	return "", fmt.Errorf("cannot determine an public DNS name or address for the API server")
}

// GetInternalDNSName gets a FQDN DNS name in ther internal network for `name`
func (kubicCfg KubicInitConfiguration) GetServiceDNSName(obj kubicutil.ObjNamespacer) string {
	domain := kubicCfg.Network.Dns.Domain
	if len(obj.GetNamespace()) > 0 {
		return fmt.Sprintf("%s.%s.svc.%s", obj.GetName(), obj.GetNamespace(), domain)
	}
	return fmt.Sprintf("%s.svc.%s", obj.GetName(), domain)
}
