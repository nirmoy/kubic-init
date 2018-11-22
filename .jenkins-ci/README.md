# Jenkins-CI:

# Goals:

Portability, well-documented, no complexity and external libraries in the pipelines.

`Simplicity`. 

# Jenkins worker usefull infos.

We can have multiples jenkins worker running our pipelines.
A jenkins worker is basically a server where  your pipeline will executed. (see upstream doc for more infos).

Important is that all jenkins workers will have the label `kubic-init` or other label that are defined in the pipelines.

For setting up a new worker read the following documentation:

# New  Worker requirements:

The new worker should have following requisites for running a kubic-init pipeline.

- terraform
- terraform-libvirt
- libvirt-devel and libvirt daemon up and running.
- proper bashrc in jenkins user (see file `bashrc`)
- libvirtd.conf well setted ( see conf file.) warning: the file is relaxed on security and is used only as an example. feel free to improve security if you wish
- golang installed ( kubic-init req.)
- docker up and running (kubic-init req.)
- java ( this is needed for the jenkins worker)
- jenkins user with home dir ( this user should belong to kvm and docker groups, and access all things needed)

## How to create a jenkins-worker quick tutorial.

0) Download the swarm plugin on the server you want to create as Jenkins worker.

https://wiki.jenkins.io/display/JENKINS/Swarm+Plugin

```bash
wget https://repo.jenkins-ci.org/releases/org/jenkins-ci/plugins/swarm-client/3.9/swarm-client-3.9.jar
mv swarm-client-3.9.jar /usr/bin
```
1) Change name and label on jenkins-worker service. Change/adapt user and password on the unit-service file
2) copy the service `jenkins-worker.service` and run it 

```bash
cp jenkins-worker.service  /etc/systemd/system/
systemctl enable jenkins-worker.service
systemctl start jenkins-worker.service
systemctl status jenkins-worker.service
```

# Pipelines

Welcome to kubic-init pipeline.

The goal of this pipelines is to be runned locally, and  independently of jenkins-server.

So you can run them by using the makefile targets.

