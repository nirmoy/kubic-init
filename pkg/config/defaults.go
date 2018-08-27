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

// CNI and network defaults
const (
	DefaultCniDriver = "flannel"

	// Default directory for CNI binaries
	DefaultCniBinDir = "/var/lib/kubelet/cni/bin"

	// Default directory for CNI configuration
	DefaultCniConfDir = "/etc/cni/net.d"

	// Default subnet for pods
	DefaultPodSubnet = "172.16.0.0/13"

	// Default subnet for services
	DefaultServiceSubnet = "172.24.0.0/16"
)

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
