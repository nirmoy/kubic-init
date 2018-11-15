#!/bin/bash

sysctl --system
systemctl daemon-reload

mkdir -p /var/lib/etcd

echo br_netfilter > /etc/modules-load.d/br_netfilter.conf

modprobe br_netfilter

sysctl -w net.ipv4.ip_forward=1

sed -i 's/driver = \"\"/driver = \"btrfs\"/' /etc/containers/storage.conf
sed -i 's|plugin_dir = \".*\"|plugin_dir = \"/var/lib/kubelet/cni/bin\"|' /etc/crio/crio.conf

echo 'runtime-endpoint: unix:///var/run/crio/crio.sock' > /etc/crictl.yaml

while ! podman load -i /tmp/${kubic_init_image} ; do echo '(will try to load the kubic-init image again)' ; sleep 5 ; done

systemctl enable --now kubic-init
