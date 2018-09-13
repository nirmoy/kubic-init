package dex

const (
	DexConfigMap = `
kind: ConfigMap
apiVersion: v1
metadata:
  name: dex
  namespace: {{ .DexNamespace }}
data:
  config.yaml: |
    issuer: "https://{{ .DexAddress }}:{{ .DexPort }}"
    storage:
      type: kubernetes
      config:
        inCluster: true
    web:
      https: 0.0.0.0:5556
      tlsCert: /etc/dex/tls/dex.crt
      tlsKey: /etc/dex/tls/dex.key
    frontend:
      dir: /usr/share/caasp-dex/web
      theme: caasp
{{ if .KubicCfg.Services.Dex.LDAP }}
    connectors:
  {{ range $Con := .KubicCfg.Services.Dex.LDAP }}
    - type: ldap
      id: {{ $Con.Id }}
      name: {{ $Con.Name }}
      config:
        host: {{ $Con.Server }}
        startTLS: {{ $Con.StartTLS }}
    {{ if and $Con.BindDN $Con.BindPW }}
        bindDN: {{ $Con.BindDN }}
        bindPW: {{ $Con.BindPW }}
    {{ else }}
        # bindDN and bindPW not present; anonymous bind will be used
    {{ end }}
        usernamePrompt: {{ $Con.UsernamePrompt }}
        rootCAData: {{ $Con.RootCAData | replace "\n" "" }}
    {{ if $Con.User.BaseDN }}
        userSearch:
          baseDN: {{ $Con.User.BaseDN }}
          filter: {{ $Con.User.Filter }}
          username: {{ index $Con.User.AttrMap "username" }}
          idAttr: {{ index $Con.User.AttrMap "id" }}
          emailAttr: {{ index $Con.User.AttrMap "email" }}
          nameAttr: {{ index $Con.User.AttrMap "name" }}
    {{ end }}
    {{ if $Con.Group.BaseDN }}
        groupSearch:
          baseDN: {{ $Con.Group.BaseDN }}
          filter: {{ $Con.Group.Filter }}
          userAttr: {{ index $Con.Group.AttrMap "user" }}
          groupAttr: {{ index $Con.Group.AttrMap "group" }}
          nameAttr: {{ index $Con.Group.AttrMap "name" }}
    {{ end }}
  {{ end }}
{{ end }}
    oauth2:
      skipApprovalScreen: true

    staticClients:
    - id: kubernetes
      redirectURIs:
      - 'urn:ietf:wg:oauth:2.0:oob'
      name: "Kubernetes"
      secret: "{{ index .DexSharedPasswords "dex-kubernetes" }}"
      trustedPeers:
      - caasp-cli
      - velum

    - id: caasp-cli
      redirectURIs:
      - 'urn:ietf:wg:oauth:2.0:oob'
      - 'http://127.0.0.1'
      - 'http://localhost'
      name: "CaaSP CLI"
      secret: "{{ index .DexSharedPasswords "dex-cli" }}"
      public: true

    - id: velum
      redirectURIs:
      - 'https://{{ .KubicCfg.Network.Dns.ExternalFqdn }}/oidc/done'
      name: "Velum"
      secret: "{{ index .DexSharedPasswords "dex-velum" }}"
`

	DexDeployment = `
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  labels:
    app: dex
    kubernetes.io/cluster-service: "true"
  name: dex
  namespace: {{ .DexNamespace }}
spec:
  selector:
    matchLabels:
      app: dex
  replicas: 3
  template:
    metadata:
      labels:
        app: dex
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
        # Kubernetes will not restart dex when the configmap or secret changes, and
        # dex will not notice anything has been changed either. By storing the checksum
        # within an annotation, we force Kubernetes to perform the rolling restart
        # of all Dex pods.
        checksum/configmap: {{ .DexConfigMapSha }}
        checksum/secret: {{ .DexCertSha }}
    spec:
      serviceAccountName: {{ .DexServiceAccount }}

      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      - key: "CriticalAddonsOnly"
        operator: "Exists"

      # ensure dex pods are running on different hosts
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 1
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - dex
              topologyKey: "kubernetes.io/hostname"

      containers:
      - image: {{ .DexImage }}
        name: dex
        command: ["/usr/bin/caasp-dex", "serve", "/etc/dex/cfg/config.yaml"]

        ports:
        - name: https
          containerPort: 5556

        readinessProbe:
          # Give Dex a little time to startup
          initialDelaySeconds: 30
          failureThreshold: 5
          successThreshold: 5
          timeoutSeconds: 10
          httpGet:
            path: /healthz
            port: https
            scheme: HTTPS

        livenessProbe:
          # Give Dex a little time to startup
          initialDelaySeconds: 30
          timeoutSeconds: 10
          httpGet:
            path: /healthz
            port: https
            scheme: HTTPS

        volumeMounts:
        - name: config
          mountPath: /etc/dex/cfg
        - name: tls
          mountPath: /etc/dex/tls
        - name: ca
          mountPath: {{ .CaCrtPath }}

      volumes:
      - name: config
        configMap:
          name: dex
          items:
          - key: config.yaml
            path: config.yaml

      - name: tls
        secret:
          secretName: {{ .DexCertsSecretName }}

      - name: ca
        hostPath:
          path: {{ .CaCrtPath }}
`
)
