# Post-control plane manifests files

- (**on build**),
  - files in this directory will be copied to `/usr/lib/kubic/manifests` in
  the `kubic-init` image.
- (**on run**, and only **in the seeder** node):
  - the `kubic-init` container will try to find all the `*.yaml` files in several directories,
  like the previously mentioned directory in the container as well as `/etc/kubic/manifests`
  in the host.
  - they will be loaded and treated as [Go templates](https://golang.org/pkg/text/template/),
    performing replacements where
       * `{{ .KubicCfg }}` is the [`KubicInitConfiguration` structure](../../pkg/config/config.go).
  - finally, they will be created/upddated in the API server.



