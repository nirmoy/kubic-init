# Description

(A proof-of-concept of) a "init" container for Kubic.

# Documentation

See the [current documentation](docs/index.md) for instructions on managing
your Kubic/Kubernetes cluster with `kubic-init`.

# Development

## Project structure

This project follows the conventions presented in the [standard Golang
project](https://github.com/golang-standards/project-layout).

## Dependencies

* `dep` (will be installed automatically if not detected)
* `go >= 1.10`

For running the `kubic-init` (either locally, in a container or in a Terraform
deployment) please make sure the `kubelet` version running in the host system
is the same `kubic-init` was compiled against. You can check current kubernetes
version in the [Gopkg.toml requirements file](Gopkg.toml).

## Building

A simple `make` should be enough. This should compile [the main
function](cmd/kubic-init/main.go) and generate a `kubic-init` binary as
well as a _Docker_ image.

## Running `kubic-init` in a Development Environment

See the ["running"](docs/devel-run.md) document for all the alternatives
for running the `kubic-init`.

### Roadmap/TODO

Before we have a functional POC we need to implement:

* [X] Development environment
* [X] Seeder
* [X] Join for nodes
  * [X] Simple joins
  * [ ] Support certificates and safer flows
* [ ] Accept/reject nodes
* [ ] Multi-master and HA
* [ ] Manage etcd in a better way (maybe with `etcdadm` or the `etcd-operator`)
* [ ] [CNI](pkg/cni)
  * [X] Load CNI manifests
  * [ ] Prepare and use an updated `flannel` image
* [X] Dex and all the other critical pods.
* [X] Use `podman` instead of Docker
* [ ] Base Kubic image
  * [ ] Install all the packages we need
  * [X] Base our kubic-init image in Tumbleweed
  * [ ] Base all container images in Tumbleweed (`hyperkube`, `etcd`...)
* [ ] All the `TODO`s in this repo...

## Bumping the Kubernetes version used by `kubic-init`

Update the constraints in `Gopkg.toml`.
