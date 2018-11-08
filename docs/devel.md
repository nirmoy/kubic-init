# Development environment for `kubic-init`

## Project structure

This project follows the conventions presented in the [standard Golang
project](https://github.com/golang-standards/project-layout).



## Building

A simple `make` should be enough. This should compile [the main
function](../cmd/kubic-init/main.go) and generate a `kubic-init` binary as
well as a _Docker_ image.

### Prerequisites

In order to build the `kubic-init` container your will need:

* `go >= 1.11`
* `docker` (for building the image)

**Important**: even when `kubic-init` is using Go 11's modules (and that would free you from the
requirement of checking out the project in `$GOPATH/src/github.com/kubic-project/kubic-init`)
the `deepcopy-gen` will not work when not inside the `$GOPATH` (see [this](https://github.com/kubernetes/code-generator/issues/21)).

### Bumping the Kubernetes version used by `kubic-init`

Update the constraints in [`go.mod`](../go.mod).




## Running `kubic-init`

### Prerequisites

For running the `kubic-init` (either locally, in a container or in a Terraform
deployment) please make sure the `kubelet` version running in the host system
is the same `kubic-init` was compiled against.

### Running in your Development Environment

There are multiple ways you can run the `kubic-init` in your development
environment, both locally (ie, in your laptop) as well as on virtual machines
(with the help of Terraform). You can even use a mix of both, for example
starting a _seeder_ in your laptop and then spawning _minions_ on VMs.

#### <a name="local"></a> Running `kubic-init` in your local machine

You can run the `kubic-init` container locally with:

1. <a name="local-run"></a> `make local-run`. This will do the following things for you:

    * build the `kubic-init` executable
    * install a [_drop-in_](../init/kubelet.drop-in.conf) unit for
    `kubelet`, so it can be started with the right parameters,
    stopping the `kubelet`.
    * run the `kubic-init` executable. This will generate a valid `kubeadm`
    configuration file, spawn a `kubeadm` process that will
    start all the control-plane containers (`etcd`, the API server,
    the controller manager and the scheduler) using the local
    container runtime.

2. `make docker-run`  will do basically the same as [`1`](#local-run) but the
   `kubic-init` will be run in a container:

    * it will prepare a `kubic-init` image
    * install the _drop-in_ (see [`1`](#local-run)).
    * run it with `docker`
      * mounting many local directories in the containar (so
      please review the `CONTAINER_VOLUMES` in the [`Makefile`](../Makefile)) 

Once you are done, you can `make local-reset`/`make docker-reset`
for stopping the control plane and removing all the leftovers.

#### <a name="terraform"></a> Running `kubic-init` with Terraform

The top-level `Makefile` includes some targets for creating a _Kubic_
cluster with the help of [Terraform](https://www.terraform.io/). All these
targets have some common steps, like:

  * starting `kubic`-based VMs
  * generating some config files from the [`cloud-init` templates](../deployments/cloud-init)
  * copying some config files and drop-in units, install packages, etc...
  * copying the `kubic-init:latest` image and load it in the CRI.
  * starting the `kubic-init` container with `podman`.

These targets can be used for creating different configurations, where we can
have combinations of a seeder and regular nodes. For example:

1. for running a **full cluster** on the VMs:
   ```bash
   $ make tf-full-run TF_ARGS="-var nodes_count=0"
   ```
   This will start a cluster with a _seeder_ and a _node_.
   You can increase the number of nodes in the cluster (or customize any
   other variable in the [Terraform file](../deployments/tf-libvirt-full/terraform.tf))
   passing some Terraform arguments:
   ```bash
   $ make tf-full-apply TF_ARGS="-var nodes_count=3"
   ```

   (see the [`f-libvirt-full`](../deployments/tf-libvirt-full)
   directory for more details).

2. for running **only a seeder** on a VM:
   ```bash
   $ make tf-seeder-run
   ```
   This will start a VM where a `kubeadm` _seeder_ will be started. This
   will use a random _token_, so you will have to look for the token in
   the logs, so maybe it will be more convenient for you to specify
   a token with:
   ```bash
   $ env TOKEN=XXXX make tf-seeder-run
   ```
   Note that at this point you could start a node in your local
   development machine (as described in [previous section](#local))
   with just:
   ```bash
   $ env SEEDER=1.2.3.4 TOKEN=XXXX make local-run
   ```
   where `1.2.3.4` would be the IP address of the _seeder_ VM.

   (see the [`tf-libvirt-nodes`](../deployments/tf-libvirt-nodes)
   directory for more details).

3. for running **only nodes** on the VMs:
   ```bash
   $ env TOKEN=XXXX make tf-nodes-run
   ```
   This will run a _one-node_ cluster (you could launch more nodes by
   setting the `nodes_count` Terraform variable, as described previously)
   that will connect to _seeder_ in the public IP address where Terraform
   is being run (you can use a different IP or name by setting the
   `SEEDER` env variable).
   
   This means that you will need to run a seeder in your local
   development machine as described in [previous section](#local).  

   (see the [`tf-libvirt-nodes`](../deployments/tf-libvirt-nodes)
   directory for more details).
   
Once you are done with your cluster, a `make tf-*-destroy` will
destroy the cluster.
