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

package config

import (
	kubeadmapiv1alpha3 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha3"
)

const (
	// DefaultKubernetesVersion Kubernetes version to deploy
	DefaultKubernetesVersion = "1.12.2"

	// DefaultAPIServerPort Default API server port
	DefaultAPIServerPort = 6443
)

const (
	// DefaultEnvVarSeeder The environment variable used for passing the seeder
	DefaultEnvVarSeeder = "SEEDER"

	// DefaultEnvVarToken The environment variable used for passing the token
	DefaultEnvVarToken = "TOKEN"

	// DefaultEnvVarManager The environment variable used for passing the kubic-manager image
	DefaultEnvVarManager = "MANAGER_IMAGE"
)

const (
	// DefaultRuntimeEngine Default runtime engine
	DefaultRuntimeEngine = "crio"
)

// DefaultCriSocket info
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

// etcd defaults
const (
	// the etcd image to use (the name `etcd` at the end is implied)
	DefaultEtdcImageRepo = "registry.opensuse.org/devel/kubic/containers/container/kubic"

	// tag of the image
	DefaultEtdcImageTag = "3.3"
)

const (
	// DefaultKubicInitImage the kubic-init image by default
	DefaultKubicInitImage = "kubic-init:latest"

	// DefaultKubeadmPath Default kubeadm path
	DefaultKubeadmPath = "/usr/bin/kubeadm"
)

// Some important default paths
const (
	// Default directory for certificates
	DefaultCertsDirectory = kubeadmapiv1alpha3.DefaultCertificatesDir

	// Default CA certificate path
	DefaultCertCA = kubeadmapiv1alpha3.DefaultCACertPath

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

	// Deafult Dex issuer port
	DefaultDexIssuerPort = 32000
)

// OIDC defaults
const (
	// Default OIDC client
	DefaultOIDCClientID = "kubernetes"

	// Default OIDC username
	DefaultOIDCUsernameClaim = "email"

	// Default OIDC groups
	DefaultOIDCGroupsClaim = "group"
)

var (
	// DefaultManifestsDirs Default directories for loading RBACs
	DefaultManifestsDirs = []string{
		"/usr/lib/kubic/config/manifests",
		"/usr/lib/kubic/manifests",
		"/usr/local/etc/kubic/config/manifests",
		"/usr/local/etc/kubic/manifests",
		"config/manifests",
	}

	// DefaultRBACDirs Default directories for loading RBACs
	DefaultRBACDirs = []string{
		"/usr/lib/kubic/config/rbac",
		"/usr/lib/kubic/rbac",
		"/usr/local/kubic/config/rbac",
		"/usr/local/kubic/rbac",
		"config/rbac",
	}

	// DefaultCRDsDirs Default directories for loading CRDs
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
	// DefaultClusterAdminRole the default admin role
	DefaultClusterAdminRole = "cluster-admin"
)

// DefaultKubeletSettings Default list of kubelet arguments
// Some of these arguments are automatically set by kubeadm
// (see https://github.com/kubernetes/kubernetes/blob/2c933695fa61d57d1c6fa5defb89caed7d49f773/cmd/kubeadm/app/phases/kubelet/flags.go#L71)
var DefaultKubeletSettings = map[string]string{
	"network-plugin": "cni",
	"cni-conf-dir":   DefaultCniConfDir,
	"cni-bin-dir":    DefaultCniBinDir,
}

// DefaultIgnoredPreflightErrors Hardcoded list of errors to ignore
var DefaultIgnoredPreflightErrors = []string{
	"Service-Docker",
	"Swap",
	"FileExisting-crictl",
	"Port-10250",
	"SystemVerification", // for ignoring docker graph=btrfs
	"IsPrivilegedUser",
}

// DefaultFeatureGates Constant set of featureGates
// A set of key=value pairs that describe feature gates for various features.
var DefaultFeatureGates = ""
