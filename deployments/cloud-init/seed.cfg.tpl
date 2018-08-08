#cloud-config

# set locale
locale: fr_FR.UTF-8

# set timezone
timezone: Europe/Paris
hostname: ${hostname}
fqdn: ${hostname}.suse.de

# set root password
chpasswd:
  list: |
    root:${password}
  expire: False

users:
  - name: qa
    gecos: User
    sudo: ["ALL=(ALL) NOPASSWD:ALL"]
    groups: users
    lock_passwd: false
    passwd: ${password}

# setup and enable ntp
ntp:
  servers:
    - ntp1.suse.de
    - ntp2.suse.de
    - ntp3.suse.de

runcmd:
  - /usr/bin/systemctl enable --now ntpd
  - sed -i -e 's/DHCLIENT_SET_HOSTNAME="yes"/DHCLIENT_SET_HOSTNAME="no"/g' /etc/sysconfig/network/dhcp

### TODO: this should be replaced by the suse_caasp module
write_files:
  - path: "/etc/caasp/caasp-init.yaml"
    permissions: "0644"
    owner: "root"
    content: |
      apiVersion: caas.suse.com/v1alpha1
      kind: CaaSInitConfiguration

### TODO: this should be replaced by the suse_caasp module
write_files:
  - path: "/etc/caasp/kubeadm.yaml"
    permissions: "0644"
    owner: "root"
    content: |
      apiVersion: kubeadm.k8s.io/v1alpha2
      kind: MasterConfiguration
      #api:
      #  advertiseAddress: __API_EXTERNAL_FQDN__
      #  bindPort: 6443
      #  controlPlaneEndpoint: ''
      #apiServerExtraArgs:
      #  feature-gates: ''
      bootstrapTokens:
        - groups:
            - 'system:bootstrappers:kubeadm:default-node-token'
          #token: __SEED_TOKEN__
          ttl: 24h0m0s
          usages:
            - signing
            - authentication
      certificatesDir: /etc/kubernetes/pki
      clusterName: kubernetes
      #etcd:
      #  image: registry.cn-hangzhou.aliyuncs.com/kubernetes-containers/etcd-amd64:latest
      #  local:
      #    dataDir: /var/lib/etcd
      #    image: ''
      featureGates:
        SelfHosting: true
        ### TODO: disable until https://github.com/kubernetes/kubeadm/issues/923
        StoreCertsInSecrets: false
        HighAvailability: true
        CoreDNS: true
      kubernetesVersion: v1.11.0
      #imageRepository: k8s.gcr.io
      #schedulerExtraArgs:
      #  # https://kubernetes.io/docs/reference/generated/kube-scheduler/
      #  feature-gates: ""
      networking:
        dnsDomain: cluster.local
        podSubnet: ''
        serviceSubnet: 10.96.0.0/12
      nodeRegistration:
        criSocket: /var/run/dockershim.sock
        name: __NODE_NAME__
      #  taints:
      #    - effect: NoSchedule
      #      key: node-role.kubernetes.io/master
      #unifiedControlPlaneImage: ''


final_message: "The system is finally up, after $UPTIME seconds"
