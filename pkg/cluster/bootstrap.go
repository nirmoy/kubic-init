package cluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
)

// RemoveAutoApprovalRBAC removes the RBAC rules created for auto approval of nodes
func RemoveAutoApprovalRBAC(client clientset.Interface) error {
	clusterRoleBinding := node.NodeAutoApproveBootstrapClusterRoleBinding

	foregroundDelete := metav1.DeletePropagationForeground
	deleteOptions := &metav1.DeleteOptions{
		PropagationPolicy: &foregroundDelete,
	}

	return client.Rbac().ClusterRoleBindings().Delete(clusterRoleBinding, deleteOptions)
}
