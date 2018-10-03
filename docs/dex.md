# External authentication 

External authentication is performed with the help of [Dex](https://github.com/dexidp/dex).
From Dex's documentation:

> Dex is an identity service that uses OpenID Connect to drive authentication for other apps.
> Dex acts as a portal to other identity providers through "connectors." This lets dex defer
authentication to LDAP servers, SAML providers, or established identity providers like
GitHub, Google, and Active Directory. Clients write their authentication logic once
to talk to dex, then dex handles the protocols for a given backend.

# Configuration

Firstly you must create a `DexConfiguration` object like this:

```yaml
# dex-config.yaml
apiVersion: kubic.opensuse.org/v1beta1
kind: DexConfiguration
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: dex-configuration
spec:
  nodePort: 32000
  adminGroup: Administrators
``` 

The name **must be** `dex-configuration` (otherwise it will be ignored).
Then you can load it with `kubectl apply -f dex-config.yaml`.

Then you can add LDAP connectors like this:

```yaml
# my-connector.yaml
apiVersion: kubic.opensuse.org/v1beta1
kind: LDAPConnector
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: external-ldap-server
spec:
  id: some-id
  name: ldap.suse.de
  server: "ldap.suse.de:389"
  user:
    baseDn: "ou=People,dc=infra,dc=caasp,dc=local"
    filter: "(objectClass=inetOrgPerson)"
    username: mail
    idAttr: DN
    emailAttr: mail
    nameAttr: cn
    group:
  group:
    baseDn: "ou=Groups,dc=infra,dc=caasp,dc=local"
    filter: "(objectClass=groupOfUniqueNames)"
    userAttr: DN
    nameAttr: cn
    groupAttr: uniqueMember
```

Once this `LDAPConnector` is loaded with `kubectl apply -f my-connector.yaml`,
a Dex `Deployment` should have been launched by the Kubic controller:

```bash
$ kubectl get deployment --all-namespaces                                                                                                dex_controller ✱ ◼
NAMESPACE     NAME        DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
kube-system   coredns     2         2         2            2           7m
kube-system   kubic-dex   3         3         3            3           3m

```

You can check  the
current status of the `DexConfiguration` by _describing_ it:
 
```bash
$ kubectl describe dexconfiguration main-configuration                                                                                   dex_controller ✱ ◼
Name:         main-configuration
Namespace:    
Labels:       controller-tools.k8s.io=1.0
Annotations:  kubectl.kubernetes.io/last-applied-configuration={"apiVersion":"kubic.opensuse.org/v1beta1","kind":"DexConfiguration","metadata":{"annotations":{},"labels":{"controller-tools.k8s.io":"1.0"},"name":"ma...
API Version:  kubic.opensuse.org/v1beta1
Kind:         DexConfiguration
Metadata:
  Creation Timestamp:  2018-10-03T15:09:05Z
  Finalizers:
    dexconfiguration.finalizers.kubic.opensuse.org
  Generation:        1
  Resource Version:  760
  Self Link:         /apis/kubic.opensuse.org/v1beta1/dexconfigurations/main-configuration
  UID:               44fbd63e-c71e-11e8-ae07-847beb0267d4
Spec:
  Admin Group:  Administrators
  Certificate:
  Node Port:  32000
Status:
  Config:      kube-system/kubic-dex
  Deployment:  kube-system/kubic-dex
  Generated Certificate:
    Name:          kubic-dex-cert
    Namespace:     kube-system
  Num Connectors:  1
  Static Passwords:
    Kubic - Dex - Cli:
      Name:       kubic-dex-cli
      Namespace:  kube-system
    Kubic - Dex - Kubernetes:
      Name:       kubic-dex-kubernetes
      Namespace:  kube-system
    Kubic - Dex - Velum:
      Name:       kubic-dex-velum
      Namespace:  kube-system
Events:
  Type    Reason     Age   From           Message
  ----    ------     ----  ----           -------
  Normal  Checking   2m    DexController  ConfigMap 'kubic-dex' for 'main-configuration' has changed
  Normal  Checking   2m    DexController  Getting certificate 'kubic-dex-cert' for 'main-configuration'...
  Normal  Checking   2m    DexController  Deployment 'kubic-dex' for 'main-configuration' has changed
  Normal  Deploying  2m    DexController  Starting/updating Dex...
  Normal  Deploying  2m    DexController  Created 3 Secrets for shared passwords for 'main-configuration'
  Normal  Deploying  2m    DexController  Configmap 'kubic-dex' created for 'main-configuration'
  Normal  Deploying  2m    DexController  Deployment 'kubic-dex' created for 'main-configuration'
```