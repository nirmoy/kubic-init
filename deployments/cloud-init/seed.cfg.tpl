#cloud-config

# set locale
locale: fr_FR.UTF-8

# set timezone
timezone: Europe/Paris
hostname: ${hostname}
fqdn: ${hostname}.suse.de

# pass some ssh pub keys
ssh_authorized_keys:
  - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDdLNLiyQ/HqWZUk6oXl5C6YAxPDLKMQoc8tL4X5K7EmQjqCBkopEaWHryvfTnrXe4U0xBxJBf6oEAFHQ/a2vCqmgVFgCdFuV8xw4SkPauf6Sbf19TWjyNC8ThHV/vxekkNT/p3Xcz+NQgDM98XqSCAZtvkpYIrf0I5g88dn+g44OGwOZM4fh227W6EevrRvNIuzaBvaOyIivGTUyle+uDCcZAEefc+nHQpq7HtJiWMDeQ9AazNEAHl3Ku9csg/rZnUiriFcuAsDIcN/JpRrdhFeivxEZfyXg2u9k8kQ+VXQ84h6gMdMLqKwofOcX6XIGtg+FnHsqv0AWcE4eUt2XzmzzFuTn70bv/cn3MyocYFQv4QIwR1wUacy9hAcuIVefuECHUybZLFJ3ycP+XJeGIMIO1kmDOr/dD4SBbZPqotJxpdLi0ZRtyFWYB7KEoEcaMSVkmCI3Kk1zTPMjy+JYM/FECuTCuprgT99Ov86udLY8UgwqS2FUBbx9Wvm5tSE4lUPIGWtIx2LzleFljT3tMZrFLKNlI2ykTjoqHsFh3OuljQHpwA3weMPVYgdHVY0unEM+MWFIDxTyj6IsCH2hhHeuqHIL2FL69BcMOAFMCBV6nd60jZ0mJh89wN/1WSSY2C1UJKMcjxi3c9mG1im0cd0Qa2Ij1zhoR3q05UHnZVDQ== kubic@ci

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
