# Bootstrapping nodes with `kubic-init`

## Some common concepts...

* Every cluster needs one (and only one) **_seeder_**.
* All the other nodes (**_minions_**) in the cluster will _join_
this seeder. So they will need the _seeder_ address.
* All the nodes in the cluster (_seeder_ and _minions_) share a
**_token_**. _minions_ cannot join the _seeder_ without this _token_.
* The _seeder_ can be started with a predefined _token_. When not provided,
the _seeder_ will generate a random one. However, the cluster Administrators
will be forced to look for the current _token_ in some way (for example,
in some user interface or command line client, in the logs or directly in
the Kubernetes secrets).  
    
* The Administrator will be responsible for creating a valid
  `kubic-init.yaml` configuration with either
  
    1) an existing _seeder_, or
    2) with no _seeder_ (that means _"this node is the seeder"_).

    _Sample configuration file_:
    
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
    generated if the Administrator checked some _"Configure as a cluster seeder"_
    checkbox (or equivalent) when setting up the machine.
    * from a `cloud-init` configuration, filling the _seeder_
     in `kubic-init.yaml` from a template.

## Flow of events on the _seeder_

* The _seeder_ will be configured as such in the `kubic-init.yaml`, either
  by YaST or by the `cloud-init` configuration.
* The OS will be bootstrapped.
* A `systemd` unit will be started.
  * The systemd unit will launch the `kubic-init` container with the
    help of `podman`. 
  * `kubic-init` will load the `kubic-init.yaml` configuration
  * `kubic-init` will check if an update is needed (see
  [the updates](#updates) section). 
  * `kubic-init` will create a corresponding `kubeadm.yaml` configuration
  file for `kubeadm`.
  * `kubic-init` will execute `kubeadm` with that configuration.
  * `kubic-init` will deploy all the kubic-specific _operators_ and critical services:
    * the Dex operator
    * the _registries_ operator
    * the Kubic cluster-api provider

## Flow of events on a _minion_

_TODO_

### Bootstrapping _minions_ with `AutoYaST`

AutoYaST can be used for bootstrapping other _minions_ from a _seeder_.

_TODO_

## <a name="updates"></a> Updates

One of the first tasks the `kubic-init` container must do is to check if a
Kubernetes update is necessary.
  
* `kubic-init` will check if there are some Kubernetes containers already
 running in this node. This detection should be based on the existence of
 static manifests for the `kubelet` (it is probably the most reliable way).
 If not running, no update will be needed.
* otherwise, get the current kubernetes components versions. This could be done
by parsing the `kubelet` manifests and extracting the images versions.
* the `kubic-init` is prepared for deploying and working with a particular version
of kubernetes, hardcoded in the binary.
  * if _current version_ < _hardcoded version_, the node will prepare a valid
  configuration for running
  [`kubeadm upgrade`](https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-upgrade/).
  * `kubic-init` will wait until the upgrade is complete
    * if no errors are detected we can assume the Kubernetes components are
    upgraded and we can continue with the regular bootstrap.  
    * if some error is detected we will
      * cleanup all leftovers
      * continue with the regular bootstrap process (hopefully we will be able to
      connect to the cluster)

