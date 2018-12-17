#!/bin/bash

# NOTE: these variables will be replaced by Terraform
IMAGE="${kubic_init_image_name}"
IMAGE_FILENAME="${kubic_init_image_tgz}"
RUNNER=${kubic_init_runner}
EXTRA_ARGS="${kubic_init_extra_args}"

###################################################################################

set_var() {
    var="$1"
    value="$2"
    file="$3"

    if [ -f $file ] ; then
        echo ">>> Setting $var=$value in $file..."
        sed -i 's|^\('"$var"'\)\ *=.*|\1 = '"$value"'|' $file
    else
        echo ">>> Creating new file $file, with $var=$value..."
        echo "$var = $value" > $file
    fi
    # TODO: add a case for where the file exists but the var doesn't
}

mkdir -p /var/lib/etcd

echo ">>> Setting up network..."
echo br_netfilter > /etc/modules-load.d/br_netfilter.conf
modprobe br_netfilter
sysctl -w net.ipv4.ip_forward=1

echo ">>> Setting up crio..."
set_var plugin_dir \"/var/lib/kubelet/cni/bin\" /etc/crio/crio.conf
echo 'runtime-endpoint: unix:///var/run/crio/crio.sock' > /etc/crictl.yaml

echo ">>> Setting up storage..."
set_var driver \"btrfs\" /etc/containers/storage.conf

echo ">>> Setting runner as $RUNNER..."
[ -x /usr/bin/$RUNNER ] || ( echo "FATAL: /usr/bin/$RUNNER does not exist!!!" ; exit 1 ; )
set_var KUBIC_INIT_RUNNER /usr/bin/$RUNNER /etc/sysconfig/kubic-init

echo ">>> Loading the kubic-init image with $RUNNER from /tmp/$IMAGE_FILENAME..."
while ! /usr/bin/$RUNNER load -i /tmp/$IMAGE_FILENAME ; do
    echo ">>> (will try to load the kubic-init image again)"
    sleep 5 
done

[ "$RUNNER" = "podman" ] && IMAGE="localhost/$IMAGE"
echo ">>> Setting image as $IMAGE"
set_var KUBIC_INIT_IMAGE "\"$IMAGE\"" /etc/sysconfig/kubic-init

if [ -n "$EXTRA_ARGS" ] ; then
    echo ">>> Setting kubic-init extra args = $EXTRA_ARGS"
    set_var KUBIC_INIT_EXTRA_ARGS "\"$EXTRA_ARGS\"" /etc/sysconfig/kubic-init
fi

echo ">>> Enabling the kubic-init service..."
sysctl --system
systemctl daemon-reload
systemctl enable kubelet

[ "$RUNNER" = "podman" ] && \
    systemctl enable --now crio || \
    systemctl enable --now docker

systemctl enable --now kubic-init
