# Development environment for `kubic-init`

## Community

Currently the Kubic-init project lives inside the kubic echosystem.

If you have a question to ask? Want to join in the disscussion? Find community information including chat and mailing lists on the main [Kubic](https://en.opensuse.org/Portal:Kubic) page.

Want to get involved but don't know what to do? Try looking at our github [issues](https://github.com/kubic-project/kubic-init/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) and use the tags `good first issue` or `help wanted`!

## Project structure

This project follows the conventions presented in the [standard Golang
project](https://github.com/golang-standards/project-layout).

## Dependencies

* `go >= 1.11`

For running the `kubic-init` (either locally, in a container or in a Terraform
deployment) please make sure the `kubelet` version running in the host system
is the same `kubic-init` was compiled against.

Note: 
We use golang modules but you still need to work inside your $GOPATH for developing `kubic-init`.
Working outside GOPATH is currently **not supported**

### Bumping the Kubernetes version used by `kubic-init`

Update the constraints in [`go.mod`](../go.mod).

## Building

A simple `make` should be enough. This should compile [the main
function](../cmd/kubic-init/main.go) and generate a `kubic-init` binary as
well as a _Docker_ image.

## Making a Pull Request

Have a PR(Pull Request) you would like to make to the project? Here are a few helpful tips to ensure that the PR moves along quickly!

 - make sure all files pass gofmt.
 - run the existing unit tests locally.
 - squash your commits into one. Our goal is to have one meaningful change per commit.
 - when making the PR, ensure you explain the reason for the PR and or link the issue it solves.

## Running `kubic-init` in your Development Environment

There are multiple ways you can run the `kubic-init` for bootstrapping
and managinig your Kubernetes cluster:

### <a name="local"></a> ... in your local machine

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

### <a name="terraform"></a> ... with Terraform

The top-level Makefile includes some targets for creating a Kubic
cluster with the help of Terraform. All these targets have some common
steps, like:

  * starting `kubic`-based VMs
  * generating some config files from the [`cloud-init` templates](../deployments/cloud-init)
  * copying some config files and drop-in units, install packages, etc...
  * copying the `kubic-init:latest` image and load it in the CRI.
  * starting the `kubic-init` container with `podman`.

These targets can be used for creating different configurations, where we can
have combinations of a seeder and regular nodes.

_Note that, in the following examples, some Terraform variables can be set with
both environment variables (ie, `TF_VAR_xxx=yyy`) or with the `-var` argument
(ie, `-var xxx=yyy`)._


1. for running a **full cluster** on the VMs:
   ```bash
   $ make tf-run
   ```
   This will start a cluster with a _seeder_ and a _node_.
   You can increase the number of nodes in the cluster (or customize any
   other variable in the [Terraform file](../deployments/terraform/terraform.tf))
   by setting the `nodes_count` variable:
   ```bash
   $ make tf-apply TF_ARGS="-var nodes_count=3"
   ```

   (see the [`terraform`](../deployments/terraform) directory for more details).

2. for running **only a seeder** on a VM:
   ```bash
   $ make tf-run TF_ARGS="-var nodes_count=0"     # or define TF_VAR_nodes_count=0
   ```
   This will start a VM where a `kubeadm` _seeder_ will be started with some
   random _token_, so you will have to look for the _token_ in
   the logs. In general, it will be more convenient to specify
   a _token_ with the `token` Terraform variable (or with the `TOKEN` env var):
   ```bash
   $ env TOKEN=XXXX make tf-run TF_ARGS="-var nodes_count=0"  # we could use '-var token="XXXX"' too
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
   $ env TOKEN=XXXX make tf-run TF_ARGS="-var seeder=localhost -var nodes_count=1"
   ```
   This will run a _one-node_ cluster (you could launch more nodes by
   setting the `nodes_count` Terraform variable, as described previously)
   that will connect to _seeder_ in the public IP address where Terraform
   is being run (you can use a different IP or name by setting the
   `SEEDER` env variable or the `seeder` Terraform variable).
   
   This means that you will need to run a seeder in your local
   development machine as described in [previous section](#local).  
   
Once you are done with your cluster, a `make tf-destroy` will
destroy the cluster.
