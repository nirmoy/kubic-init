
[![CircleCI](https://circleci.com/gh/kubic-project/kubic-init/tree/master.svg?style=svg)](https://circleci.com/gh/kubic-project/kubic-init/tree/master)

# Description

A "init" container for [Kubic](https://en.opensuse.org/Portal:Kubic).

# Documentation

See the [current documentation](docs/README.md) for instructions on managing
your Kubic cluster with `kubic-init`.

# Development

See the [development documentation](docs/devel.md).

### Roadmap/TODO

Before we have a functional POC we need to implement:

* [X] Development environment
* [ ] Cluster formation workflow
  * [X] Seeder
  * [X] Join for nodes
    * [X] Simple joins
    * [ ] Support certificates and safer flows
  * [ ] Accept/reject nodes
  * [ ] Add/remove nodes once the cluster is up and running
    * [ ] Node addition
      * [ ] Masters
      * [X] Workers
    * [ ] Node removal
      * [ ] Masters
      * [ ] Workers
* [ ] Command line interface
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
  * [ ] Base all container images in Tumbleweed
    * [ ] `hyperkube`
    * [ ] `etcd`
    * [ ] `coredns`
* [ ] All the `TODO`s in this repo...
