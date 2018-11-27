# Some CI around Jenkins-pipelines:

Welcome to kubic-init pipeline.

The goal of this pipelines is to be runned locally, and  independently of jenkins-server.

The only requirement are currently:

`terraform`, `terraform-libvirt-plugin` and libvirt-client plus a kvm server where you will need to have the vms.

# Portability:

By design this pipelines will not rely on external plugin and/or jenknins library.

# How to create a jenkins-worker quick tutorial.

0) Download the swarm plugin on the server you want to create as Jenkins worker.

https://wiki.jenkins.io/display/JENKINS/Swarm+Plugin

```bash
wget https://repo.jenkins-ci.org/releases/org/jenkins-ci/plugins/swarm-client/3.9/swarm-client-3.9.jar
```

0.B) Make sure you have the jenkins user with the right credentials etc. (look at upstream doc)

1) Run the swarm plugin on the server you want to make ci-server with ( adapt this command with your PWD and user, and all the vars)

```bash
/usr/bin/java -jar /usr/bin/swarm-client-3.9.jar -master https://ci.suse.de/ -disableSslVerification -disableClientsUniqueId -name kubic-ci -description "CI runner used by the kubic" -username containers -password BauBaus -labels kubic-init -executors 3 -mode exclusive -fsroot /home/jenkins/build -deleteExistingClients 
```

This will connect the Jenkins worker to the master server.

2) You can also create a systemd service. ( in case you reboot the worker is usefull)

```bash
[Unit] Description=swarmplugin After=network.target [Service] User=jenkins EnvironmentFile=/etc/sysconfig/swarmplugin ExecStart=/usr/bin/java -Djava.util.logging.config.file=/usr/share/java/logging-swarm-client.properties -jar /usr/share/java/swarm-client-jar-with-dependencies.jar -master https://ci.suse.de/ -username BAUBAU -password BAUPWD -labels BAULABEL -executors 4 -disableSslVerification -name kubic-init -fsroot/home/jenkins/jenkins-build/ Restart=always [Install] WantedBy=multi-user.target
```
