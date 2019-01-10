
## 0.0.3 Initial kubic-init
### Kubic-init
- Check if the kubic-init must reset or bootstrap before going forward
- cni: make default cni config placement configurable
- cni: add cilium cni plugin
- cni: flannel initialization and make the CNI configuration and binaries dirs configurable
- config: parse bind interface
- Use Flannel as default CNI
- Load the kubic-manager once the seeder has finished with the control-plane
- When `autoApproval=false` in the config file, remove the RBAC rules used for approvinfg nodes automatically.
- Use leap15 as docker image
- Use the etcd image from the registry.suse.de

### K8s & Kubeadm
- Add a command for getting kubeadm's version and log it.
- Upgrade to k8s 1.13.0
- Use the v1beta1 API
- Use admin kubeconfig to deploy CRD's
- Use a higher level kubeadm API. Get some config from environment variables.
- Honor the DynamicKubeletConfig feature gate
