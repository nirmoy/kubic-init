
## 0.0.5 Initial kubic-init
### Kubic-init
- Check if the kubic-init must reset or bootstrap before going forward
- cni: make default cni config placement configurable
- cni: add cilium cni plugin
- Fix Flannel initialization Make the CNI configuration and binaries dirs configurable
- config: parse bind interface
- CNI drivers, using Flannel as the default one.

### K8s & Kubeadm
- Add a command for getting kubeadm's version and log it.
- Upgrade to k8s 1.13.0
- Use the v1beta1 API
- Use leap15 as docker image
- Use the etcd image from the registry.suse.de
- Use admin kubeconfig to deploy CRD's
- Use a higher level kubeadm API and get rid of our custom stages. Get some config from environment variables.      
- Honor the DynamicKubeletConfig feature gate


### CI
- Add basic validation via circleci
- Add packaging kubic-init script for concourse job
- Add ssh-keys for ci
- Updated makefile  to report golint errors and CI errors
- add coverage make target
- add PR template
- enable parallelism and workflow
- adapt Pipeline and targets for basic deploy
- Add skeleton for ci-minimal pipe
- introduce terraform-fmt for syntax check
- adapt cloudinit with 0.5 version
