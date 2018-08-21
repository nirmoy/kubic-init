FROM opensuse:tumbleweed

ARG EXTRA_REPO0="https://download.opensuse.org/repositories/devel:/kubic/openSUSE_Tumbleweed/"
#ARG EXTRA_REPO1="https://download.opensuse.org/repositories/devel:/CaaSP:/Head:/ControllerNode/openSUSE_Leap_15.0/"
ARG RUN_RPMS="docker-kubic kubernetes-client kubernetes-kubeadm cri-tools ca-certificates iptables systemd"
ARG KUBIC_INIT_EXE="cmd/kubic-init/kubic-init"
ARG KUBIC_INIT_SH="build/image/entrypoint.sh"

### Install some packages we need for running kubeadm
RUN \
  zypper ar --refresh --enable --no-gpgcheck ${EXTRA_REPO0} extra-repo0 && \
  zypper ref -r extra-repo0 && \
  zypper in -y --no-recommends ${RUN_RPMS}

### Prepare the container for running docker and kubeadm
RUN cd /usr/lib/systemd/system/sysinit.target.wants/ && \
  for i in * ; do [ "${i##*/}" = "systemd-tmpfiles-setup.service" ] || rm -f "$i" ; done ; \
  rm -f /lib/systemd/system/multi-user.target.wants/* ; \
  rm -f /etc/systemd/system/*.wants/* ; \
  rm -f /usr/lib/systemd/system/local-fs.target.wants/* ; \
  rm -f /usr/lib/systemd/system/sockets.target.wants/*udev*  ; \
  rm -f /usr/lib/systemd/system/sockets.target.wants/*initctl*  ; \
  rm -f /usr/lib/systemd/system/basic.target.wants/* ; \
  rm -rf /var/run/docker.sock

### TODO: do not build the kubic-init exec IN this container:
###       maybe we will use the OBS and this whole Dockerfile
###       will be gone...
COPY $KUBIC_INIT_EXE /usr/local/bin/kubic-init
COPY $KUBIC_INIT_SH /usr/local/bin/kubic-init.sh
RUN chmod 755 /usr/local/bin/kubic-init*

### Directories we will mount from the host
VOLUME /sys/fs/cgroup

CMD /usr/local/bin/kubic-init.sh
