#!/bin/bash

# NOTE: these variables will be replaced by Terraform
IMAGE="${kubic_init_image_name}"
IMAGE_FILENAME="${kubic_init_image_tgz}"
RUNNER=${kubic_init_runner}
EXTRA_ARGS="${kubic_init_extra_args}"

###################################################################################

log() { echo ">>> $@" ; }

set_var() {
    var="$1"
    value="$2"
    file="$3"

    if [ -f $file ] ; then
        log "Setting $var=$value in $file..."
        sed -i 's|^\('"$var"'\)\ *=.*|\1 = '"$value"'|' $file
    else
        log "Creating new file $file, with $var=$value..."
        echo "$var = $value" > $file
    fi
    # TODO: add a case for where the file exists but the var doesn't
}

print_file() {
    log "Contents of $1:"
    log "------------------------------------"
    cat $1 | awk '{ print ">>> " $0 }'
    log "------------------------------------"
}

log "Making sure kubic-init is not running..."
systemctl stop kubic-init  >/dev/null 2>&1 || /bin/true

log "Setting runner as $RUNNER..."
[ -x /usr/bin/$RUNNER ] || ( log "FATAL: /usr/bin/$RUNNER does not exist!!!" ; exit 1 ; )
set_var KUBIC_INIT_RUNNER /usr/bin/$RUNNER /etc/sysconfig/kubic-init

log "Removing any previous kubic-init image..."
/usr/bin/$RUNNER rmi kubic-init  >/dev/null 2>&1 || /bin/true

log "Setting up network..."
echo br_netfilter > /etc/modules-load.d/br_netfilter.conf
modprobe br_netfilter
sysctl -w net.ipv4.ip_forward=1

log "Setting up crio..."
set_var plugin_dir \"/var/lib/kubelet/cni/bin\" /etc/crio/crio.conf
echo 'runtime-endpoint: unix:///var/run/crio/crio.sock' > /etc/crictl.yaml

log "Setting up storage..."
set_var driver \"btrfs\" /etc/containers/storage.conf

[ "$RUNNER" = "podman" ] && IMAGE="localhost/$IMAGE"
log "Setting image as $IMAGE"
set_var KUBIC_INIT_IMAGE "\"$IMAGE\"" /etc/sysconfig/kubic-init

if [ -n "$EXTRA_ARGS" ] ; then
    log "Setting kubic-init extra args = $EXTRA_ARGS"
    set_var KUBIC_INIT_EXTRA_ARGS "\"$EXTRA_ARGS\"" /etc/sysconfig/kubic-init
fi

print_file /etc/sysconfig/kubic-init
print_file /etc/kubic/kubic-init.yaml

[ -d /var/lib/etcd ] || ( log "Creating etcd staorage..." ;  mkdir -p /var/lib/etcd ; )

log "Loading the kubic-init image with $RUNNER from /tmp/$IMAGE_FILENAME..."
while ! /usr/bin/$RUNNER load -i /tmp/$IMAGE_FILENAME ; do
    log "(will try to load the kubic-init image again)"
    sleep 5
done

log "Enabling the kubic-init service..."
sysctl --system
systemctl daemon-reload
systemctl enable kubelet

[ "$RUNNER" = "podman" ] && \
    systemctl enable --now crio || \
    systemctl enable --now docker

systemctl enable --now kubic-init
