package flannel

import (
	"fmt"
	"net"

	"github.com/ereslibre/kubic-init/pkg/cni"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
)

const (
	// FlannelClusterRoleName sets the name for the flannel ClusterRole
	// TODO: This k8s-generic, well-known constant should be fetchable from another source, not be in this package
	FlannelClusterRoleName = "suse:kubic:flannel"

	// FlannelServiceAccountName describes the name of the ServiceAccount for the flannel addon
	FlannelServiceAccountName = "flannel"

	FlannelImage = "sles12/flannel:0.9.1"
)

// EnsureFlannelAddon creates the flannel addons
func EnsureFlannelAddon(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	if err := CreateServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating flannel service account: %v", err)
	}

	cidr_ip, cidr_net, err := net.ParseCIDR(cfg.Networking.PodSubnet)
	if err != nil {
		return fmt.Errorf("could not parse Pod CIDR: %v", err)
	}
	cidr_len, _ := cidr_net.Mask.Size()

	var proxyConfigMapBytes, proxyDaemonSetBytes []byte
	proxyConfigMapBytes, err = kubeadmutil.ParseTemplate(FlannelConfigMap19,
		struct {
			Network string
			SubnetLen  int
			Backend  string
		}{
			cidr_ip.String(),
			cidr_len,
			"vxlan", // TODO: replace by some config arg
		})
	if err != nil {
		return fmt.Errorf("error when parsing flannel configmap template: %v", err)
	}

	proxyDaemonSetBytes, err = kubeadmutil.ParseTemplate(FlannelDaemonSet19,
		struct{
			Image string
			LogLevel int
			HealthzPort int
			ConfDir string
			BinDir string
		}{
			FlannelImage,
			1, // TODO: replace by some config arg
			8471,
			cni.DefaultConfDir,
			cni.DefaultBinDir,
		})
	if err != nil {
		return fmt.Errorf("error when parsing flannel daemonset template: %v", err)
	}
	if err := createFlannelAddon(proxyConfigMapBytes, proxyDaemonSetBytes, client); err != nil {
		return err
	}
	if err := CreateRBACRules(client); err != nil {
		return fmt.Errorf("error when creating flannel RBAC rules: %v", err)
	}

	fmt.Println("[addons] Applied essential addon: flannel")
	return nil
}

// CreateServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func CreateServiceAccount(client clientset.Interface) error {

	return apiclient.CreateOrUpdateServiceAccount(client, &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      FlannelServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		},
	})
}

// CreateRBACRules creates the essential RBAC rules for a minimally set-up cluster
func CreateRBACRules(client clientset.Interface) error {
	return createClusterRoleBindings(client)
}

func createFlannelAddon(configMapBytes, daemonSetbytes []byte, client clientset.Interface) error {
	kubeproxyConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configMapBytes, kubeproxyConfigMap); err != nil {
		return fmt.Errorf("unable to decode flannel configmap %v", err)
	}

	// Create the ConfigMap for flannel or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, kubeproxyConfigMap); err != nil {
		return err
	}

	kubeproxyDaemonSet := &apps.DaemonSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), daemonSetbytes, kubeproxyDaemonSet); err != nil {
		return fmt.Errorf("unable to decode flannel daemonset %v", err)
	}

	// Create the DaemonSet for flannel or update it in case it already exists
	return apiclient.CreateOrUpdateDaemonSet(client, kubeproxyDaemonSet)
}

func createClusterRoleBindings(client clientset.Interface) error {
	return apiclient.CreateOrUpdateClusterRoleBinding(client, &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:node-proxier",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     FlannelClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      FlannelServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	})
}
