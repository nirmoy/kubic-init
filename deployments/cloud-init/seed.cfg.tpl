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
  - /usr/bin/systemctl enable --now ntpd || bin/true
  - sed -i -e 's/DHCLIENT_SET_HOSTNAME="yes"/DHCLIENT_SET_HOSTNAME="no"/g' /etc/sysconfig/network/dhcp
  - echo PermitRootLogin yes >> /etc/ssh/sshd_config
  - systemctl restart sshd

### TODO: this should be replaced by a "kubic" module
write_files:
  - path: "/etc/kubic/kubic-init.yaml"
    permissions: "0644"
    owner: "root"
    content: |
      apiVersion: kubic.suse.com/v1alpha1
      kind: KubicInitConfiguration
      clusterFormation:
        token: ${token}
      manager:
        image: ${kubic_init_image_name}

final_message: "The system is finally up, after $UPTIME seconds"
