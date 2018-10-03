# Running `kubic-init` in your Development Environment

You have several ways of running the `kubic-init`.

1. You can run the `kubic-init` container locally with a
`make docker-run`. This will:

   * build the `kubic-init` image
   * install a [_drop-in_](init/kubelet.drop-in.conf) unit for
   kubelet, so it can be started with the right parameters.
   * stop the `kubelet`
   * run it with `docker`
     * using the config files in `/configs`
     * mounting many local directories in the containar (so
     please review the `CONTAINER_VOLUMES` in the [`Makefile`](Makefile))
   * start the `kubelet`
   * start all the control-plane containers (etcd, the API server,
   the controller manager and the scheduller) in the local
   `docker` daemon.

   Once you are done, you can `make docker-reset` for stopping the
   control plane and removing all the leftovers.

2. You can run the container as specified in `1`. and then use this
instance as a _seeder_ for new nodes that are started in VMs with
the help of Terraform. You can start these nodes with a
`make tf-nodes-run`. This will:

   * start Kubic-based VMs, generating some config files from
   the [`cloud-init` templates](deployments/cloud-init)
   * copy some config files and drop-in units, install packages, etc...
   * copy the `kubic-init:latest` image and load it in the CRI.
   * start the `kubic-init` container from a CRI _systemd_ unit.

   Do a `make tf-nodes-destroy` once you are done.
   See the `deployments/tf-libvirt-nodes` directory for more details.

3. Very similar to `2`,  but instead of starting only the nodes,
you can start all the machines (the seeder and the nodes) with Terraform
with `make tf-full-run`.

