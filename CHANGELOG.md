
## 0.0.3 Initial kubic-init - January 12 2019

### Kubic-init

- Add check if the kubic-init must reset or bootstrap before going forward (https://github.com/kubic-project/kubic-init/pull/149)

- cni: make default cni config placement configurable
- cni: add cilium cni plugin
- cni: flannel initialization and make the CNI configuration and binaries dirs configurable
- config: parse bind interface (https://github.com/kubic-project/kubic-init/pull/142)
- cni: use flannel as the default CNI driver
- Load the kubic-manager once the seeder has finished with the control-plane
- When `autoApproval=false` in the config file, remove the RBAC rules used for approvinfg nodes automatically.
- Use the etcd image from the registry.suse.de

### K8s & Kubeadm
- Add a command for getting kubeadm's version and log it.
- Upgrade to k8s 1.13.0
- Use the v1beta1 API
- Use admin kubeconfig to deploy CRD's
- Get rid of our own Init/Join code (https://github.com/kubic-project/kubic-init/pull/2)
