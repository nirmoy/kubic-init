# `kubic-init` deployments

IaaS, PaaS, system and container orchestration deployment configurations and
templates (`terraform`).

# Requirements:

Try to use always  the latest version of [terraform-libvirt](https://github.com/dmacvicar/terraform-provider-libvirt/releases) and `terraform` upstream.

# Several deployments:

* A complete cluster with all the nodes running the `kubic-init` container ([`tf-libvirt-full`](tf-libvirt-full)).
* A seeder-only cluster with the node running the `kubic-init` container ([`tf-libvirt-full`](tf-libvirt-full) with `nodes_count = 0`).
* A only-nodes cluster ([`tf-libvirt-nodes`](tf-libvirt-nodes)), using the localhost as the seeder.
