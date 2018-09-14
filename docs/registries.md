# Adding Docker registries

In order to use images comming from secure repositories other
than the official Docker Hub, users must install the certificate
for those repositories in all the machines in the cluster.

That operation can be automated with the help of the Kubic manager.

# Configuration

Firstly you must load the certyifciate for your registry to a `Secret`. For example,
`CA.crt` for the registry `registry.suse.de`, we can upload that CRT with:

```bash
$ kubectl create secret generic suse-ca-crt --from-file=ca.crt=/etc/pki/trust/anchors/SUSE_CaaSP_CA.crt
```

(note well that the filename must be `ca.crt` in the `--from-file` argument)

Then you must create a registry definition, specifying the `<host>:<port>` of the
registry as well as a reference to the `Secret` where we stored the certificate.

```yaml
# suse-registry.yaml
apiVersion: "kubic.opensuse.org/v1beta1"
kind: Registry
metadata:
  name: suse-registry
  namespace: kube-system
spec:
  hostPort: "registry.suse.de:5000"
  # secret with the ca.crt used for pulling images from this registry
  certificate:
    name: suse-ca-crt
    namespace: kube-system
```

Once this object is loaded with `kubectl apply -f suse-registry.yaml`, the certificate
will be automatically populated to all the nodes in the cluster.
