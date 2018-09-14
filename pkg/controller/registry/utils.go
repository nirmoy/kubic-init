package registry

import (
	"context"
	"crypto/md5"
	"encoding/base64"

	"github.com/golang/glog"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getAllNodes gets the list of nodes in the cluster
func getAllNodes(r client.Client) (map[string]*corev1.Node, error) {
	glog.V(5).Infof("[kubic] getting the list of nodes...")
	nodes := &corev1.NodeList{}
	if err := r.List(context.TODO(), &client.ListOptions{}, nodes); err != nil {
		glog.V(1).Infof("[kubic] error when getting the list of Nodes in the cluster: %s", err)
		return nil, err
	}
	glog.V(5).Infof("[kubic] %d nodes in the cluster", len(nodes.Items))

	res := map[string]*corev1.Node{}
	for _, node := range nodes.Items {
		res[node.Name] = &node
	}
	return res, nil
}

// getAllJobsWithLabels gets the list of jobs in the cluster
func getAllJobsWithLabels(r client.Client, labels map[string]string) ([]batchv1.Job, error) {
	glog.V(5).Infof("[kubic] getting the list of jobs...")
	jobs := &batchv1.JobList{}

	// only return the Jobs that were created by the 'kubic-registry-installer'
	listOptions := &client.ListOptions{}
	listOptions.MatchingLabels(labels)

	if err := r.List(context.TODO(), listOptions, jobs); err != nil {
		glog.V(1).Infof("[kubic] error when getting the list of Jobs in the cluster: %s", err)
		return nil, err
	}
	return jobs.Items, nil
}

// getSecretHash gets the Hash for the CA.crt in a Secret
// (we must return a printable string, so we will use the base64)
func getSecretHash(secret *corev1.Secret) string {
	if secret == nil {
		glog.V(5).Infof("[kubic] no secret provided: empty hash")
		return ""
	}
	crt, found := secret.Data["ca.crt"]
	if !found {
		glog.V(5).Infof("[kubic] no CA.crt in Secret '%s'", secret.Name)
		return ""
	}

	b := md5.Sum(crt)
	hashStr := base64.RawURLEncoding.EncodeToString(b[:])
	return hashStr
}
