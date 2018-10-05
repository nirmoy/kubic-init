# `cloud-init` profiles

The `cloud-init` configuration files in this directorry can be used
as an example for your deployments. 

Note well that you must have a valid configuration file for the
`kubic-init` container. You can achieve this by adding a
`write_files` statement like this:

```
write_files:
  - path: "/etc/kubic/kubic-init.yaml"
    permissions: "0644"
    owner: "root"
    content: |
      apiVersion: kubic.suse.com/v1alpha1
      kind: KubicInitConfiguration
      clusterFormation:
        seeder: ${seeder}
        token: ${token}

```
