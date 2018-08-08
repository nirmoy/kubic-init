package bootstrap

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/preflight"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	utilsexec "k8s.io/utils/exec"
)

/**
 * Join an existing cluster
 *
 * This is basically a properly packaged version of the code found in
 * https://github.com/kubernetes/kubernetes/blob/release-1.11/cmd/kubeadm/app/cmd/join.go
 */
func Join(nodeCfg *kubeadmapi.NodeConfiguration, ignorePreflightErrorsSet sets.String) error {
	var err error

	// When joining, kubeadm uses the Bootstrap Token credential perform a
	// TLS bootstrap, which fetches the credential needed to download the kubelet-config-1.X
	// ConfigMap and writes it to /var/lib/kubelet/config.yaml. The dynamic environment file
	// is generated in exactly the same way as kubeadm init.
	//
	// Next, kubeadm runs the following two commands to load the new configuration into the kubelet:
	//
	// systemctl daemon-reload && systemctl restart kubelet
	//
	// After the kubelet loads the new configuration, kubeadm writes the
	// /etc/kubernetes/bootstrap-kubelet.conf KubeConfig file, which contains a CA certificate
	// and Bootstrap Token. These are used by the kubelet to perform the TLS Bootstrap and
	// obtain a unique credential, which is stored in /etc/kubernetes/kubelet.conf. When this
	// file is written, the kubelet has finished performing the TLS Bootstrap.

	fmt.Println("[preflight] running pre-flight checks")

	// Then continue with the others...
	glog.V(1).Infoln("[preflight] running various checks on all nodes")
	if err = preflight.RunJoinNodeChecks(utilsexec.New(), nodeCfg, ignorePreflightErrorsSet); err != nil {
		return err
	}

	bootstrapClient, err := PhaseBootstrapKubeNode(nodeCfg)
	if err != nil {
		return err
	}

	if err = PhaseNodeKubeletReconfig(nodeCfg, bootstrapClient); err != nil {
		return err
	}

	// Now the kubelet will perform the TLS Bootstrap,
	// transforming /etc/kubernetes/bootstrap-kubelet.conf to /etc/kubernetes/kubelet.conf
	// Wait for the kubelet to create the /etc/kubernetes/kubelet.conf KubeConfig file.
	// If this process times out, display a somewhat user-friendly message.
	waiter := apiclient.NewKubeWaiter(nil, kubeadmconstants.TLSBootstrapTimeout, os.Stdout)
	if err = WaitForKubeletAndFunc(waiter, WaitForTLSBootstrappedClient); err != nil {
		return err
	}

	// When we know the /etc/kubernetes/kubelet.conf file is available, get the client
	client, err := kubeconfigutil.ClientSetFromFile(kubeadmconstants.GetKubeletKubeConfigPath())
	if err != nil {
		return err
	}

	if err = PhasePatchNode(&nodeCfg.NodeRegistration, client); err != nil {
		return err
	}

	// if features.Enabled(nodeCfg.FeatureGates, features.DynamicKubeletConfig)
	if err = PhaseKubeletDynamicConf(&nodeCfg.NodeRegistration, client); err != nil {
		return err
	}

	return nil
}
