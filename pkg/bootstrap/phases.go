package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/discovery"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	dnsaddonphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	proxyaddonphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
	clusterinfophase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/clusterinfo"
	nodebootstraptokenphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
	certsphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"
	controlplanephase "k8s.io/kubernetes/cmd/kubeadm/app/phases/controlplane"
	etcdphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/etcd"
	kubeconfigphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	kubeletphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubelet"
	markmasterphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/markmaster"
	patchnodephase "k8s.io/kubernetes/cmd/kubeadm/app/phases/patchnode"
	selfhostingphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/selfhosting"
	uploadconfigphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/uploadconfig"
	"k8s.io/kubernetes/cmd/kubeadm/app/preflight"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	utilsexec "k8s.io/utils/exec"
)

/**
 * Generate the certificates
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#generate-the-necessary-certificates
 */
func PhaseCerts(cfg *kubeadmapi.MasterConfiguration) error {
	// certsDirToWriteTo is gonna equal cfg.CertificatesDir in the normal case, but gonna be a temp directory if dryrunning
	// TODO: check the certsDir
	realCertsDir := cfg.CertificatesDir
	kubeConfigDir := kubeadmconstants.KubernetesDir

	if res, _ := certsphase.UsingExternalCA(cfg); !res {

		// PHASE 1: Generate certificates
		glog.V(1).Infof("[init] creating PKI Assets")
		if err := certsphase.CreatePKIAssets(cfg); err != nil {
			return err
		}

		// PHASE 2: Generate kubeconfig files for the admin and the kubelet
		glog.V(2).Infof("[init] generating kubeconfig files")
		if err := kubeconfigphase.CreateInitKubeConfigFiles(kubeConfigDir, cfg); err != nil {
			return err
		}

	} else {
		fmt.Println("[externalca] the file 'ca.key' was not found, yet all other certificates are present. Using external CA mode - certificates or kubeconfig will not be generated")
	}

	// Temporarily set cfg.CertificatesDir to the "real value" when writing controlplane manifests
	// This is needed for writing the right kind of manifests
	cfg.CertificatesDir = realCertsDir
	return nil
}

/**
 * Generate all the control plane static manifests
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#generate-static-pod-manifests-for-control-plane
 */
func PhaseControlPlane(cfg *kubeadmapi.MasterConfiguration) error {
	manifestDir := kubeadmconstants.GetStaticPodDirectory()

	// Bootstrap the control plane
	glog.V(1).Infof("[init] bootstraping the control plane")
	glog.V(1).Infof("[init] creating static pod manifest")
	if err := controlplanephase.CreateInitStaticPodManifestFiles(manifestDir, cfg); err != nil {
		return fmt.Errorf("error creating init static pod manifest files: %v", err)
	}
	return nil
}

/**
 * Create a a static manifest for starting etcd
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#generate-static-pod-manifest-for-local-etcd
 */
func PhaseEtcd(cfg *kubeadmapi.MasterConfiguration) error {
	manifestDir := kubeadmconstants.GetStaticPodDirectory()

	// Add etcd static pod spec only if external etcd is not configured
	if cfg.Etcd.External == nil {
		glog.V(1).Infof("[init] no external etcd found. Creating manifest for local etcd static pod")
		if err := etcdphase.CreateLocalEtcdStaticPodManifestFile(manifestDir, cfg); err != nil {
			return fmt.Errorf("error creating local etcd static pod manifest file: %v", err)
		}
	}
	return nil
}

/**
 * Set up the node bootstrap tokens
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#configure-tls-bootstrapping-for-node-joining
 */
func PhaseNodeBootstrap(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {

	tokens := []string{}
	for _, bt := range cfg.BootstrapTokens {
		tokens = append(tokens, bt.Token.String())
	}
	if len(tokens) == 1 {
		fmt.Printf("[bootstraptoken] using token: %s\n", tokens[0])
	} else if len(tokens) > 1 {
		fmt.Printf("[bootstraptoken] using tokens: %v\n", tokens)
	}

	// Create the default node bootstrap token
	glog.V(1).Infof("[init] creating RBAC rules to generate default bootstrap token")
	if err := nodebootstraptokenphase.UpdateOrCreateTokens(client, false, cfg.BootstrapTokens); err != nil {
		return fmt.Errorf("error updating or creating token: %v", err)
	}
	// Create RBAC rules that makes the bootstrap tokens able to post CSRs
	glog.V(1).Infof("[init] creating RBAC rules to allow bootstrap tokens to post CSR")
	if err := nodebootstraptokenphase.AllowBootstrapTokensToPostCSRs(client); err != nil {
		return fmt.Errorf("error allowing bootstrap tokens to post CSRs: %v", err)
	}
	// Create RBAC rules that makes the bootstrap tokens able to get their CSRs approved automatically
	glog.V(1).Infof("[init] creating RBAC rules to automatic approval of CSRs automatically")
	if err := nodebootstraptokenphase.AutoApproveNodeBootstrapTokens(client); err != nil {
		return fmt.Errorf("error auto-approving node bootstrap tokens: %v", err)
	}

	// Create/update RBAC rules that makes the nodes to rotate certificates and get their CSRs approved automatically
	glog.V(1).Infof("[init] creating/updating RBAC rules for rotating certificate")
	if err := nodebootstraptokenphase.AutoApproveNodeCertificateRotation(client); err != nil {
		return err
	}
	return nil
}

/**
 * The self hosting phase basically replaces static Pods for control plane
 * components with DaemonSets
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#optional-and-alpha-in-v19-self-hosting
 */
func PhaseSelfHost(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	kubeConfigDir := kubeadmconstants.KubernetesDir
	manifestDir := kubeadmconstants.GetStaticPodDirectory()

	// make the control plane self-hosted if feature gate is enabled
	// if features.Enabled(cfg.FeatureGates, features.SelfHosting) {

	glog.V(1).Infof("[init] feature gate is enabled. Making control plane self-hosted")
	// Temporary control plane is up, now we create our self hosted control
	// plane components and remove the static manifests:
	fmt.Println("[self-hosted] creating self-hosted control plane")

	timeout := 4 * time.Minute
	waiter := apiclient.NewKubeWaiter(client, timeout, os.Stdout)

	if err := selfhostingphase.CreateSelfHostedControlPlane(manifestDir, kubeConfigDir, cfg, client, waiter, false); err != nil {
		return fmt.Errorf("error creating self hosted control plane: %v", err)
	}

	return nil
}

/**
 * Stop the kubelet, write a new configuration and try to start it again
 */
func PhaseKubeletReconfig(cfg *kubeadmapi.MasterConfiguration) error {
	kubeletDir := kubeadmconstants.KubeletRunDirectory

	// First off, configure the kubelet. In this short timeframe, kubeadm is trying to stop/restart the kubelet
	// Try to stop the kubelet service so no race conditions occur when configuring it
	glog.V(1).Infof("Stopping the kubelet")
	preflight.TryStopKubelet()

	// Write env file with flags for the kubelet to use. We do not need to write the --register-with-taints for the master,
	// as we handle that ourselves in the markmaster phase
	// TODO: Maybe we want to do that some time in the future, in order to remove some logic from the markmaster phase?
	if err := kubeletphase.WriteKubeletDynamicEnvFile(&cfg.NodeRegistration, cfg.FeatureGates, false, kubeletDir); err != nil {
		return fmt.Errorf("error writing a dynamic environment file for the kubelet: %v", err)
	}

	// Write the kubelet configuration file to disk.
	if err := kubeletphase.WriteConfigToDisk(cfg.KubeletConfiguration.BaseConfig, kubeletDir); err != nil {
		return fmt.Errorf("error writing kubelet configuration to disk: %v", err)
	}

	// Try to start the kubelet service in case it's inactive
	glog.V(1).Infof("Starting the kubelet")
	preflight.TryStartKubelet()
	return nil
}

/**
 * Stop the kubelet in a node that is bootstrapping, write a new configuration and try to start it again
 */
func PhaseNodeKubeletReconfig(cfg *kubeadmapi.NodeConfiguration, client clientset.Interface) error {
	// First off, configure the kubelet. In this short timeframe, kubeadm is trying to stop/restart the kubelet
	// Try to stop the kubelet service so no race conditions occur when configuring it
	glog.V(1).Infof("Stopping the kubelet")
	preflight.TryStopKubelet()

	// This is the procedure in nodes
	kubeletVersion, err := preflight.GetKubeletVersion(utilsexec.New())
	if err != nil {
		return err
	}

	// Write the configuration for the kubelet (using the bootstrap token credentials)
	// to disk so the kubelet can start
	if err := kubeletphase.DownloadConfig(client, kubeletVersion, kubeadmconstants.KubeletRunDirectory); err != nil {
		return err
	}

	// Write env file with flags for the kubelet to use. Also register taints
	if err := kubeletphase.WriteKubeletDynamicEnvFile(&cfg.NodeRegistration, cfg.FeatureGates, true, kubeadmconstants.KubeletRunDirectory); err != nil {
		return err
	}

	// Try to start the kubelet service in case it's inactive
	glog.V(1).Infof("Starting the kubelet")
	preflight.TryStartKubelet()
	return nil
}

/**
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#optional-and-alpha-in-v19-write-init-kubelet-configuration
 */
func PhaseKubeletDynamicConf(nr *kubeadmapi.NodeRegistrationOptions, client clientset.Interface) error {
	kubeletVersion, err := preflight.GetKubeletVersion(utilsexec.New())
	if err != nil {
		return err
	}

	// Enable dynamic kubelet configuration for the node.
	if err := kubeletphase.EnableDynamicConfigForNode(client, nr.Name, kubeletVersion); err != nil {
		return fmt.Errorf("error enabling dynamic kubelet configuration: %v", err)
	}
	return nil
}

/**
 *
 */
func PhaseKubeletCreateConfig(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	glog.V(1).Infof("[init] creating kubelet configuration configmap")
	if err := kubeletphase.CreateConfigMap(cfg, client); err != nil {
		return fmt.Errorf("error creating kubelet configuration ConfigMap: %v", err)
	}
	return nil
}

/**
 *
 */
func PhaseBootstrapKubeNode(nodeCfg *kubeadmapi.NodeConfiguration) (*clientset.Clientset, error) {
	// Perform the Discovery, which turns a Bootstrap Token and optionally (and preferably) a CA cert hash into a KubeConfig
	// file that may be used for the TLS Bootstrapping process the kubelet performs using the Certificates API.
	glog.V(1).Infoln("[join] retrieving KubeConfig objects")
	kubeconfigCfg, err := discovery.For(nodeCfg)
	if err != nil {
		return nil, err
	}

	bootstrapKubeConfigFile := kubeadmconstants.GetBootstrapKubeletKubeConfigPath()

	// Write the bootstrap kubelet config file or the TLS-Boostrapped kubelet config file down to disk
	glog.V(1).Infoln("[join] writing bootstrap kubelet config file at", bootstrapKubeConfigFile)
	if err = kubeconfigutil.WriteToDisk(bootstrapKubeConfigFile, kubeconfigCfg); err != nil {
		return nil, fmt.Errorf("couldn't save bootstrap-kubelet.conf to disk: %v", err)
	}

	// Write the ca certificate to disk so kubelet can use it for authentication
	cluster := kubeconfigCfg.Contexts[kubeconfigCfg.CurrentContext].Cluster
	if err = certutil.WriteCert(nodeCfg.CACertPath, kubeconfigCfg.Clusters[cluster].CertificateAuthorityData); err != nil {
		return nil, fmt.Errorf("couldn't save the CA certificate to disk: %v", err)
	}

	bootstrapClient, err := kubeconfigutil.ClientSetFromFile(bootstrapKubeConfigFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't create client from kubeconfig file %q: %v", bootstrapKubeConfigFile, err)
	}

	return bootstrapClient, nil
}

/**
 * Upload currently used configuration to the cluster
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#saves-kubeadm-masterconfiguration-in-a-configmap-for-later-reference
 */
func PhaseUploadConfig(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	// Note: This is done right in the beginning of cluster initialization; as we might want to make other phases
	// depend on centralized information from this source in the future
	glog.V(1).Infof("[init] uploading currently used configuration to the cluster")
	if err := uploadconfigphase.UploadConfiguration(cfg, client); err != nil {
		return fmt.Errorf("error uploading configuration: %v", err)
	}
	return nil
}

/**
 * Mark the master with the right label/taint
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#mark-master
 */
func PhaseMarkMaster(nr *kubeadmapi.NodeRegistrationOptions, client clientset.Interface) error {
	glog.V(1).Infof("[init] marking the master with right label")
	if err := markmasterphase.MarkMaster(client, nr.Name, nr.Taints); err != nil {
		return fmt.Errorf("error marking master: %v", err)
	}
	return nil
}

func PhasePatchNode(nr *kubeadmapi.NodeRegistrationOptions, client clientset.Interface) error {
	glog.V(1).Infof("[init] preserving the crisocket information")
	if err := patchnodephase.AnnotateCRISocket(client, nr.Name, nr.CRISocket); err != nil {
		return fmt.Errorf("error uploading crisocket: %v", err)
	}
	return nil
}

/**
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#create-the-public-cluster-info-configmap
 */
func PhaseClusterInfo(client clientset.Interface) error {
	kubeConfigDir := kubeadmconstants.KubernetesDir

	// Create the cluster-info ConfigMap with the associated RBAC rules
	glog.V(1).Infof("[init] creating bootstrap configmap")
	adminKubeConfigPath := filepath.Join(kubeConfigDir, kubeadmconstants.AdminKubeConfigFileName)
	if err := clusterinfophase.CreateBootstrapConfigMapIfNotExists(client, adminKubeConfigPath); err != nil {
		return fmt.Errorf("error creating bootstrap configmap: %v", err)
	}
	glog.V(1).Infof("[init] creating ClusterInfo RBAC rules")
	if err := clusterinfophase.CreateClusterInfoRBACRules(client); err != nil {
		return fmt.Errorf("error creating clusterinfo RBAC rules: %v", err)
	}
	return nil
}

/**
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#install-dns-addon
 */
func PhaseAddonDNS(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	glog.V(1).Infof("[init] ensuring DNS addon")
	if err := dnsaddonphase.EnsureDNSAddon(cfg, client); err != nil {
		return fmt.Errorf("error ensuring dns addon: %v", err)
	}
	return nil
}

/**
 * See https://github.com/kubernetes/kubeadm/blob/0840c14900073b74a59891fb53c85a5587314124/docs/design/design_v1.10.md#install-kube-proxy-addon
 */
func PhaseAddonProxy(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	glog.V(1).Infof("[init] ensuring proxy addon")
	if err := proxyaddonphase.EnsureProxyAddon(cfg, client); err != nil {
		return fmt.Errorf("error ensuring proxy addon: %v", err)
	}
	return nil
}

func WaitAPIServer(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	// waiter holds the apiclient.Waiter implementation of choice, responsible for querying the API server in various ways and waiting for conditions to be fulfilled
	glog.V(1).Infof("[init] waiting for the API server to be healthy")

	timeout := 4 * time.Minute
	waiter := apiclient.NewKubeWaiter(client, timeout, os.Stdout)

	fmt.Printf("[init] waiting for the kubelet to boot up the control plane as Static Pods from directory %q \n", kubeadmconstants.GetStaticPodDirectory())
	fmt.Println("[init] this might take a minute or longer if the control plane images have to be pulled")

	if err := WaitForKubeletAndFunc(waiter, waiter.WaitForAPI); err != nil {
		ctx := map[string]string{
			"Error":                  fmt.Sprintf("%v", err),
			"APIServerImage":         images.GetCoreImage(kubeadmconstants.KubeAPIServer, cfg.GetControlPlaneImageRepository(), cfg.KubernetesVersion, cfg.UnifiedControlPlaneImage),
			"ControllerManagerImage": images.GetCoreImage(kubeadmconstants.KubeControllerManager, cfg.GetControlPlaneImageRepository(), cfg.KubernetesVersion, cfg.UnifiedControlPlaneImage),
			"SchedulerImage":         images.GetCoreImage(kubeadmconstants.KubeScheduler, cfg.GetControlPlaneImageRepository(), cfg.KubernetesVersion, cfg.UnifiedControlPlaneImage),
		}

		// Set .EtcdImage conditionally
		if cfg.Etcd.Local != nil {
			ctx["EtcdImage"] = fmt.Sprintf("				- %s", images.GetCoreImage(kubeadmconstants.Etcd, cfg.ImageRepository, cfg.KubernetesVersion, cfg.Etcd.Local.Image))
		} else {
			ctx["EtcdImage"] = ""
		}

		return fmt.Errorf("couldn't initialize a Kubernetes cluster")
	}

	return nil
}

// waitForKubeletAndFunc waits primarily for the function f to execute, even though it might take some time. If that takes a long time, and the kubelet
// /healthz continuously are unhealthy, kubeadm will error out after a period of exponential backoff
func WaitForKubeletAndFunc(waiter apiclient.Waiter, f func() error) error {
	errorChan := make(chan error)

	go func(errC chan error, waiter apiclient.Waiter) {
		// This goroutine can only make kubeadm init fail. If this check succeeds, it won't do anything special
		// TODO: Make 10248 a constant somewhere
		if err := waiter.WaitForHealthyKubelet(40*time.Second, "http://localhost:10248/healthz"); err != nil {
			errC <- err
		}
	}(errorChan, waiter)

	go func(errC chan error, waiter apiclient.Waiter) {
		// This main goroutine sends whatever the f function returns (error or not) to the channel
		// This in order to continue on success (nil error), or just fail if the function returns an error
		errC <- f()
	}(errorChan, waiter)

	// This call is blocking until one of the goroutines sends to errorChan
	return <-errorChan
}

// waitForTLSBootstrappedClient waits for the /etc/kubernetes/kubelet.conf file to be available
func WaitForTLSBootstrappedClient() error {
	fmt.Println("[tlsbootstrap] Waiting for the kubelet to perform the TLS Bootstrap...")

	kubeletKubeConfig := filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.KubeletKubeConfigFileName)
	// Loop on every falsy return. Return with an error if raised. Exit successfully if true is returned.
	return wait.PollImmediate(kubeadmconstants.APICallRetryInterval, kubeadmconstants.TLSBootstrapTimeout, func() (bool, error) {
		_, err := os.Stat(kubeletKubeConfig)
		return (err == nil), nil
	})
}
