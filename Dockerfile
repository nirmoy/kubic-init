FROM opensuse:tumbleweed

ARG KUBIC_INIT_EXE="cmd/kubic-init/kubic-init"

ARG EXTRA_REPO0="https://download.opensuse.org/repositories/devel:/kubic/openSUSE_Tumbleweed/"

ARG RUN_RPMS="cri-tools iptables iproute2 systemd kubernetes-kubeadm"

ENV SYSTEMCTL_FORCE_BUS 1
ENV DBUS_SYSTEM_BUS_ADDRESS unix:path=/var/run/dbus/system_bus_socket

RUN \
  zypper ar --refresh --enable --no-gpgcheck ${EXTRA_REPO0} extra-repo0 && \
  zypper ref -r extra-repo0 && \
  zypper in -y --no-recommends ${RUN_RPMS} && \
  zypper clean -a

# Copy stuff to the image...
# (check the .dockerignore file for exclusions)

### TODO: do not build the kubic-init exec IN this container:
###       maybe we will use the OBS and this whole Dockerfile
###       will be gone...
COPY $KUBIC_INIT_EXE /usr/local/bin/kubic-init
RUN chmod 755 /usr/local/bin/kubic-init*

# Copy all the static files
ADD config/crds      /usr/lib/kubic/crds
ADD config/rbac      /usr/lib/kubic/rbac
ADD config/manifests /usr/lib/kubic/manifests

### Directories we will mount from the host
VOLUME /sys/fs/cgroup

CMD /usr/local/bin/kubic-init
