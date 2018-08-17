
# Description

(A proof-of-concept of) a "init" container for Kubic.

# Development

## Project structure

This project follows the conventions presented in https://github.com/golang-standards/project-layout.

## Development

### Dependencies

* `dep` (will be installed automatically if not detected)
* `go >= 1.10`

### Building

A simple `make` should be enough. This should compile [the main
function](cmd/kubic-init/main.go) and generate a `kubic-init` binary as
well as a _Docker_ image.

### Running `kubic-init`

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
   * copy the `kubic-init:latest` image and load it in the Docker daemon.
   * start the `kubic-init` container from a Docker-based
   _systemd_ unit.  

   Do a `make tf-nodes-destroy` once you are done.   
   See the `deployments/tf-libvirt-nodes` directory for more details.

3. Very similar to `2`,  but instead of starting only the nodes,
you can start all the machines (the seeder and the nodes) with Terraform
with `make tf-full-run`.

### Roadmap/TODO

Before we have a functional POC we need to implement:

* [X] Development environment
* [X] Seeder
* [ ] Join for nodes
* [ ] Accept/reject nodes
* [ ] [CNI](pkg/cni), Dex and all the other critical pods.
* [ ] Use `podman` instead of Docker
* [ ] Install some requirements in the base Kubic images
* [ ] All the `TODO`s in this repo...

### Bumping the Kubernetes version used by `kubic-init`

Update the constraints in `Gopkg.toml`.

