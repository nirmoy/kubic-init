package config

import "k8s.io/kubernetes/cmd/kubeadm/app/features"

const (
	// Default API server port
	DefaultAPIServerPort = 6443
)

const (
	// The environment variable used for passing the seeder
	DefaultEnvVarSeeder = "SEEDER"

	// The environment variable used for passing the token
	DefaultEnvVarToken = "TOKEN"
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
