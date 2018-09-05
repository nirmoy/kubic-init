package flannel

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"

	"github.com/kubic-project/kubic-init/pkg/cni"
	"github.com/kubic-project/kubic-init/pkg/config"
)

const (
	// FlannelClusterRoleName sets the name for the flannel ClusterRole
	// TODO: This k8s-generic, well-known constant should be fetchable from another source, not be in this package
	FlannelClusterRoleName = "suse:kubic:flannel"

	// FlannelServiceAccountName describes the name of the ServiceAccount for the flannel addon
	FlannelServiceAccountName = "flannel"

	FlannelHealthPort = 8471
)

func init() {
	// self-register in the CNI plugins registry
	cni.Registry.Register("flannel", EnsureFlannelAddon)
}

// EnsureFlannelAddon creates the flannel addons
func EnsureFlannelAddon(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	if err := CreateServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating flannel service account: %v", err)
	}

	var flannelConfigMapBytes, flannelDaemonSetBytes []byte
	flannelConfigMapBytes, err := kubeadmutil.ParseTemplate(FlannelConfigMap19,
		struct {
			Network   string
			Backend   string
		}{
			cfg.Network.PodSubnet,
			"vxlan", // TODO: replace by some config arg
		})

	if err != nil {
		return fmt.Errorf("error when parsing flannel configmap template: %v", err)
	}

	flannelDaemonSetBytes, err = kubeadmutil.ParseTemplate(FlannelDaemonSet19,
		struct {
			Image       string
			LogLevel    int
			HealthzPort int
			ConfDir     string
			BinDir      string
		}{
			cfg.Network.Cni.Image,
			1, // TODO: replace by some config arg
			FlannelHealthPort,
			config.DefaultCniConfDir,
			config.DefaultCniBinDir,
		})

	if err != nil {
		return fmt.Errorf("error when parsing flannel daemonset template: %v", err)
	}

	if err := createFlannelAddon(flannelConfigMapBytes, flannelDaemonSetBytes, client); err != nil {
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
	flannelConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configMapBytes, flannelConfigMap); err != nil {
		return fmt.Errorf("unable to decode flannel configmap %v", err)
	}

	// Create the ConfigMap for flannel or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, flannelConfigMap); err != nil {
		return err
	}

	flannelDaemonSet := &apps.DaemonSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), daemonSetbytes, flannelDaemonSet); err != nil {
		return fmt.Errorf("unable to decode flannel daemonset %v", err)
	}

	// Create the DaemonSet for flannel or update it in case it already exists
	return apiclient.CreateOrUpdateDaemonSet(client, flannelDaemonSet)
}

func createClusterRoleBindings(client clientset.Interface) error {
	return apiclient.CreateOrUpdateClusterRoleBinding(client, &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: FlannelClusterRoleName,
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
