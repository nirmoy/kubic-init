package bootstrap

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeadmapi         "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmapiv1alpha2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha2"
	bto "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	"k8s.io/kubernetes/cmd/kubeadm/app/preflight"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	utilsexec "k8s.io/utils/exec"
)

func Init(cfg *kubeadmapiv1alpha2.MasterConfiguration, masterCfg *kubeadmapi.MasterConfiguration, bto *bto.BootstrapTokenOptions, ignorePreflightErrorsSet sets.String) error {
	//
	// When starting the seeder, the .kubeletConfiguration.baseConfig structure is marshalled
	// to disk at /var/lib/kubelet/config.yaml, and also uploaded to a ConfigMap in the cluster.
	// The ConfigMap is named kubelet-config-1.X, where .X is the minor version of the
	// Kubernetes version you are initializing. A kubelet configuration file is also written
	// to /etc/kubernetes/kubelet.conf with the baseline cluster-wide configuration for all
	// kubelets in the cluster. This configuration file points to the client certificates that
	// allow the kubelet to communicate with the API server. This addresses the need to
	// propogate cluster-level configuration to each kubelet.
	//
	// To address the second pattern of providing instance-specific configuration details,
	// kubeadm writes an environment file to /var/lib/kubelet/kubeadm-flags.env, which
	// contains a list of flags to pass to the kubelet when it starts. The flags are
	// presented in the file like this:
	//
	// KUBELET_KUBEADM_ARGS="--flag1=value1 --flag2=value2 ..."
	//
	// In addition to the flags used when starting the kubelet, the file also contains dynamic
	// parameters such as the cgroup driver and whether to use a different CRI runtime socket
	// (--cri-socket).
	//
	// After marshalling these two files to disk, kubeadm attempts to run the following two
	// commands, if you are using systemd:
	//
	// systemctl daemon-reload && systemctl restart kubelet
	//
	// If the reload and restart are successful, the normal kubeadm init workflow continues.

	var err error

	err = bto.ApplyTo(cfg)
	kubeadmutil.CheckErr(err)

	glog.V(1).Infof("[init] validating feature gates")
	err = features.ValidateVersion(features.InitFeatureGates, masterCfg.FeatureGates, masterCfg.KubernetesVersion)
	kubeadmutil.CheckErr(err)

	fmt.Printf("[init] using Kubernetes version: %s\n", masterCfg.KubernetesVersion)

	fmt.Println("[preflight] running pre-flight checks")
	err = preflight.RunInitMasterChecks(utilsexec.New(), masterCfg, ignorePreflightErrorsSet)
	kubeadmutil.CheckErr(err)

	fmt.Println("[preflight/images] Pulling images required for setting up a Kubernetes cluster")
	fmt.Println("[preflight/images] This might take a minute or two, depending on the speed of your internet connection")
	fmt.Println("[preflight/images] You can also perform this action in beforehand using 'kubeadm config images pull'")

	err = preflight.RunPullImagesCheck(utilsexec.New(), masterCfg, ignorePreflightErrorsSet)
	kubeadmutil.CheckErr(err)

	// Get directories to write files to; can be faked if we're dry-running
	glog.V(1).Infof("[init] Getting certificates directory from configuration")
	certsDirToWriteTo := cfg.CertificatesDir

	if err = PhaseKubeletReconfig(masterCfg); err != nil {
		return err
	}

	if err = PhaseCerts(masterCfg); err != nil {
		return err
	}

	if err = PhaseControlPlane(masterCfg); err != nil {
		return err
	}

	if err = PhaseEtcd(masterCfg); err != nil {
		return err
	}

	// Revert the earlier CertificatesDir assignment to the directory that can be written to
	cfg.CertificatesDir = certsDirToWriteTo

	// Create a kubernetes client and wait for the API server to be healthy
	glog.V(1).Infof("creating Kubernetes client")
	client, err := kubeconfigutil.ClientSetFromFile(kubeadmconstants.GetAdminKubeConfigPath())
	if err != nil {
		return err
	}

	if err = WaitAPIServer(masterCfg, client); err != nil {
		return err
	}

	if err = PhaseUploadConfig(masterCfg, client); err != nil {
		return err
	}

	if err = PhaseKubeletCreateConfig(masterCfg, client); err != nil {
		return err
	}

	if err = PhaseMarkMaster(&masterCfg.NodeRegistration, client); err != nil {
		return err
	}

	if err = PhasePatchNode(&masterCfg.NodeRegistration, client); err != nil {
		return err
	}

	// if features.Enabled(cfg.FeatureGates, features.DynamicKubeletConfig) {
	if err = PhaseKubeletDynamicConf(&masterCfg.NodeRegistration, client); err != nil {
		return err
	}

	if err = PhaseNodeBootstrap(masterCfg, client); err != nil {
		return err
	}

	if err = PhaseClusterInfo(client); err != nil {
		return err
	}

	if err = PhaseAddonDNS(masterCfg, client); err != nil {
		return err
	}

	if err = PhaseAddonProxy(masterCfg, client); err != nil {
		return err
	}

	if err = PhaseSelfHost(masterCfg, client); err != nil {
		return err
	}

	return  nil
}
