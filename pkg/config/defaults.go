package config

import (
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
)

const (
	// Default API server port
	DefaultAPIServerPort = 6443
)

const (
	// The environment variable used for passing the seeder
	DefaultEnvVarSeeder = "SEEDER"

	// The environment variable used for passing the token
	DefaultEnvVarToken = "TOKEN"

	// The environment variable used for passing the kubic-manager image
	DefaultEnvVarManager = "MANAGER_IMAGE"
)

const (
	// Default runtime engine
	DefaultRuntimeEngine = "crio"
)

var DefaultCriSocket = map[string]string{
	"docker":     "/var/run/dockershim.sock",
	"crio":       "/var/run/crio/crio.sock",
	"containerd": "/var/run/containerd/containerd.sock",
}

// CNI and network defaults
const (
	DefaultCniDriver = "flannel"

	DefaultCniImage = "registry.opensuse.org/devel/caasp/kubic-container/container/kubic/flannel:0.9.1"

	// Default directory for CNI binaries
	DefaultCniBinDir = "/var/lib/kubelet/cni/bin"

	// Default directory for CNI configuration
	DefaultCniConfDir = "/etc/cni/net.d"

	// Default subnet for pods
	DefaultPodSubnet = "172.16.0.0/13"

	// Default subnet for services
	DefaultServiceSubnet = "172.24.0.0/16"

	// Default internal DNS name
	DefaultDNSDomain = "cluster.local"
)

// Some important default paths
const (
	// Default directory for certificates
	DefaultCertsDirectory = "/etc/kubernetes/pki"

	// Directory with manifests files to load in the API server once
	// the control plane is up and runningg
	DefaultPostControlPlaneManifestsDir = "/etc/kubic/manifests"
)

// Some Kubic defaults
const (
	// the kubic-init image by default
	DefaultKubicInitImage = "kubic-init:latest"

	// The kubic-init entrypoint
	DefaultKubicInitExeInstallPath = "/usr/local/bin/kubic-init"

	// The default config file
	DefaultKubicInitConfig = "/etc/kubic/kubic-init.yaml"

	// The ConfigMap where the config file (from the Seeder) is stored
	DefaultKubicInitConfigmap = "kubic-init-config-seeder"

	// The default manifests dirctory
	DefaultKubicManifestsDir = "/etc/kubic/manifests"

	// The default CRDs dirctory
	DefaultKubicCRDDir = "/etc/kubic/crds"

	// The default RBAC dirctory
	DefaultKubicRBACDir = "/etc/kubic/rbac"

	// The default kubeconfig
	DefaultKubicKubeconfig = "/etc/kubernetes/admin.conf"
)

var (
	// Default directories for loading RBACs
	DefaultManifestsDirs = []string{
		"/usr/lib/kubic/config/manifests",
		"/usr/lib/kubic/manifests",
		"/usr/local/etc/kubic/config/manifests",
		"/usr/local/etc/kubic/manifests",
		"config/manifests",
	}

	// Default directories for loading RBACs
	DefaultRBACDirs = []string{
		"/usr/lib/kubic/config/rbac",
		"/usr/lib/kubic/rbac",
		"/usr/local/kubic/config/rbac",
		"/usr/local/kubic/rbac",
		"config/rbac",
	}

	// Default directories for loading CRDs
	DefaultCRDsDirs = []string{
		"/usr/lib/kubic/config/crds",
		"/usr/lib/kubic/crds",
		"/usr/local/kubic/config/crds",
		"/usr/local/kubic/crds",
		"config/crds",
	}
)

// k8s permissions, groups and RBAC defaults
const (
	DefaultClusterAdminRole = "cluster-admin"
)

// Default list of kubelet arguments
// Some of these arguments are automatically set by kubeadm
// (see https://github.com/kubernetes/kubernetes/blob/2c933695fa61d57d1c6fa5defb89caed7d49f773/cmd/kubeadm/app/phases/kubelet/flags.go#L71)
var DefaultKubeletSettings = map[string]string{
	"network-plugin": "cni",
	"cni-conf-dir":   DefaultCniConfDir,
	"cni-bin-dir":    DefaultCniBinDir,
}

// Hardcoded list of errors to ignore
var DefaultIgnoredPreflightErrors = []string{
	"Service-Docker",
	"Swap",
	"FileExisting-crictl",
	"Port-10250",
	"IsPrivilegedUser",
}

// Constant set of featureGates
// A set of key=value pairs that describe feature gates for various features.
var DefaultFeatureGates = (features.CoreDNS + "=true," +
	features.HighAvailability + "=false," +
	features.SelfHosting + "=true," +
	// TODO: disabled until https://github.com/kubernetes/kubeadm/issues/923
	features.StoreCertsInSecrets + "=false," +
	// TODO: results in some errors... needs some research
	features.DynamicKubeletConfig + "=false")
