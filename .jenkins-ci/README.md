# Some CI around Jenkins-pipelines:

Welcome to kubic-init pipeline.

# Description

The goal of this pipelines is to be runned locally, and  independently of jenkins-server.

The only requirement are currently:

`terraform`, `terraform-libvirt-plugin` and libvirt-client plus a kvm server where you will need to have the vms.

# Portability:

By design this pipelines will not rely on external plugin and/or jenknins library.

