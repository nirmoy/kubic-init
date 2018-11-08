# Bootstrapping nodes with `kubic-init`

## Some common concepts...

* Every cluster needs one (and only one) _seeder_.

  * All the other nodes (_minions_) in the cluster will _join_
    this seeder. So they will need the _seeder_ address.
  * All the nodes in the cluster (_seeder_ and _minions_) share a
    _token_. _minions_ cannot join the _seeder_ without this token.
    * The _seeder_ can be started with a specific _token_. Otherwise,
    it will generate a random _token_.
    
* The operator will be responsible for creating a valid
  `kubic-init.yaml` configuration with either
  
    1) an existing _seeder_, or
    2) with no _seeder_ at all.

    Sample configuration file:
    
    ```yaml  
     apiVersion: kubic.suse.com/v1alpha2
     kind: KubicInitConfiguration
     clusterFormation:
       # the seeder for the cluster formation
       # when no seeder is specified, this node will be the seeder
       seeder: some-node.com
       # token for both discovery-token and tls-bootstrap-token
       token: 94dcda.c271f4ff502789ca
       # nodes with the right token will be automatically approved
       autoApprove: false    
    ```
    
  This configuration file could be created:
  
    * by YaST from a template, where no _seeder_ would be
    generated if the operator checked some _"Configure as a cluster seeder"_
    checkbox (or equivalent) when setting up the machine.
    * from a `cloud-init` configuration, filling the _seeder_
     in `kubic-init.yaml` from a template.

## Flow of events on the _seeder_

* The _seeder_ will be configured as such in the `kubic-init.yaml`, either
  by YaST or by the `cloud-init` configuration.
* The OS will be bootstrapped.
* A `systemctd` unit will be started.
  * The systemd unit will launch the `kubic-init` container with the
    help of `podman`. 
  * `kubic-init` will load the `kubic-init.yaml` configuration and create
    a corresponding `kubeadm.yaml` configuration file for `kubeadm`.
  * `kubic-init` will execute `kubeadm` with that configuration.
