# Description

IaaS, PaaS, system and container orchestration deployment configurations and
templates (docker-compose, kubernetes/helm, mesos, terraform, bosh).

Several deployments:

* A complete cluster will all the nodes runnign the `caasp-init` container (`tf-libvirt-full`).
* A only-nodes cluster (`tf-libvirt-nodes`), using the localhost as the seeder.
