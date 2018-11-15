# Description

Terraform script for running a Kubic cluster with
all the machines as VMs.

## Discovery

Cluster formation is based on a simple token shared between all the VMs
in the cluster. This token is created by Terraform with the help of a
`external` resource.


## Graph Visualisation.

You can use https://github.com/28mm/blast-radius for visualizing the terraform resource/module dependencies in an interactive way
