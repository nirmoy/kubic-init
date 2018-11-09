# Updating the cluster

_TODO_

## Objectives

The update mechanism will have to:

* keep up-to-date the packages installed in the base OS
* update the `kubic` containers or operators running in the cluster

while preserving some constraints, like

* not disrupting the normal operation of the cluster. This imposes some
constraints like not updating more that one master node at the same time.
* being able to block updates on some nodes (like database nodes)
or on some conditions (like node _labels_) until the Administrator allows
the update.
* being able to resists failures in updates.

There are some leassons learned from the past like:

* we don't know what an update will look like. that means that
  we must be prepared for running tasks at any possible moment, like
  a _"cluster-wide pre-update migration"_ tasks as well as
  a _"per-node post reboot"_ task.

  _TODO_: _some examples_





  
## Cluster-wide updates

### Description

It would be based on the cluster-api update mechanism.

_TODO_

### User commands for managing updates

* User would have to use the cluster API tools for triggering updates

_TODO: some examples..._

### Self-update

_TODO_








## the _Kubic node updater_ container

### Overview

* This container would be responsible for checking if new packages are available in
  the `zypper` repositories, as well as new versions of the `kubic-init` software.
* An _Kubic node updater_ container would need to be run on all the nodes in the cluster. 
  * It will be started as a `DaemonSet`. That will make things easier
    when trying to update it: we will just [trigger a rolling-update of
    the `DaemonSet`](https://kubernetes.io/docs/tasks/manage-daemon/update-daemon-set/).
  * This container very likely run in privileged mode.

### Basic internals

The _Kubic node updater_ will check periodically if an update is available by:
  * checking if `zypper` reports any new software (by checking the output
      of `zypper dup --dry-run` or equivalent).
  * checking if `<current-kubic-init>` version has changed. This could be
    done in different ways:
    * by quering the SUSE _registry_ (see [\[1\]](#no-connectivity)).
    * by refreshing the RPM repos and checking if some update is available.
    * or maybe using some combination of both.

<a name="no-connectivity"></a>\[1\] _we must also consider the case where the
cluster has no internet connectivity. In that case we will probably have to
fallback to a "only packages" solution._
 
When an update is available

  * it will add some kind of _"some updates available"_  label to the `Node` in the
  kubernetes API server. It could add some other extra labels like:
    * a _"reboot will be needed"_ label
    * a _"security fix"_ label
    * a _"kubic containers update available"_ or _"base OS update available"_ label
    * ...  
  * once the lable has been set, it will wait until some kind of _"please go ahead"_ 
  label is assigned to `Node`. This could be `assigned manually by the Administrator or
  automatically by some _controller_ (they should take into account some constraints like
  availability, etc...).

#### Updating the `kubic-init` container

If the `kubic-init` is going to be updated:

* set some kind of _"kubic-init update started at = <current time>"_ label on this _Node_.  
* save the current configuration, versioned, in some well-know location in the filesystem.
* `pull` that specific version of `kubic-init` image.
* save the current version.
* restart the `kubic-init` container.
* cleanup some labels
* wait until the _Node_ is labeled with a _"kubic-init version running"_

#### Updating the base OS

If a base OS update is going to be performed:

* set some kind of _"base OS update started at = <current time>"_ label on this _Node_.  
* run some kind of `zypper up`/`transaction-update up` in the host.
* determine if the node must be rebooted.
  * it will always be necessary when using `transaction-update`
  * for `zypper` updates we will have to get that information from the packages.
* in case we must reboot the machine (ie, the kernel has been updated), we assume is
reboot is allowed.
* cleanup some labels

### Extra labels

The _Kubic node updater_ will also gather some information on start about the Node and create some
corresponding labels like:

* the base OS version/release (ie, `micro-os-release=4.0`)
* the `kubic-init` container version currently running
* ...

This information can be used by the updates controller for improving the updates planning.

## Implementation suggestions:

* a new `kubic-update` operator could be created in the `kubic-project` namespace.
* a new `kubic-update` executable will provide two entrypoints:
  * `kubic-update node` will run the _Kubic node updater_ for a nodes. It will be deployed as
  a `DaemonSet` by the controller on initialization, by creating something like:
 
    ```yaml
    apiVersion: extensions/v1beta1
    kind: DaemonSet
    metadata:
      name: kubic-update
    spec:
      template:
        metadata:
          labels:
            name: kubic-update
        spec:
          hostNetwork: true
          containers:
            - image: registry.opensuse.org/kubic-update:2.0
              name: kubic-update
              imagePullPolicy: IfNotPresent
              securityContext:
                privileged: true
              args:
              - node
          ...
    ``` 
    (note: the controller would set the right version in the
    `registry.opensuse.org/kubic-update:2.0` image).
    The _Kubic node updater_ should have [tolerations](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/#taints-and-tolerations)
    for nodes that are unscheduable, not-ready and so... 

 
## Future steps

* Perform snapshots for the rollback upgrade use case
* Snapshot etcd
* Snapshot the kube-system namespace manifests

