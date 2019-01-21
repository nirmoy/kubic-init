/*
 * Copyright 2018 SUSE LINUX GmbH, Nuernberg, Germany..
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package cilium

const (
	// CiliumCniConfigMap the config map for cni config
	CiliumCniConfigMap = `
kind: ConfigMap
apiVersion: v1
metadata:
  name: cni-config
  namespace: kube-system
  labels:
    tier: node
    app: cilium
data:
  cni-conf.json: |
    {
        "name": "cilium",
        "type": "cilium-cni"
    }
`

	// CiliumEtcdConfigMap the config map for etcd credentials
	CiliumEtcdConfigMap = `
kind: ConfigMap
apiVersion: v1
metadata:
  name: cilium-config
  namespace: kube-system
  labels:
    tier: node
    app: cilium
data:
  # This etcd-config contains the etcd endpoints of your cluster. If you use
  # TLS please make sure you uncomment the ca-file line and add the respective
  # certificate has a k8s secret, see explanation below in the comment labeled
  # "ETCD-CERT"
  etcd-config: |-
    ---
    endpoints:
    - https://{{ .EtcdServer }}:2379
    #
    # In case you want to use TLS in etcd, uncomment the following line
    # and add the certificate as explained in the comment labeled "ETCD-CERT"
    ca-file: '/tmp/cilium-etcd/ca.crt'
    #
    # In case you want client to server authentication, uncomment the following
    # lines and add the certificate and key in cilium-etcd-secrets below
    key-file: '/tmp/cilium-etcd/tls.key'
    cert-file: '/tmp/cilium-etcd/tls.crt'

  # If you want to run cilium in debug mode change this value to true
  debug: "false"
  disable-ipv4: "false"

`
	// CiliumDaemonSet cilium deamon set
	CiliumDaemonSet = `
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: {{ .DaemonSetName}}
  namespace: kube-system
spec:
  updateStrategy:
    type: "RollingUpdate"
    rollingUpdate:
      # Specifies the maximum number of Pods that can be unavailable during the update process.
      # The current default value is 1 or 100% for daemonsets; Adding an explicit value here
      # to avoid confusion, as the default value is specific to the type (daemonset/deployment).
      maxUnavailable: "100%"
  selector:
    matchLabels:
      k8s-app: cilium
      kubernetes.io/cluster-service: "true"
  template:
    metadata:
      labels:
        k8s-app: cilium
        kubernetes.io/cluster-service: "true"
      annotations:
        # This annotation plus the CriticalAddonsOnly toleration makes
        # cilium to be a critical pod in the cluster, which ensures cilium
        # gets priority scheduling.
        # https://kubernetes.io/docs/tasks/administer-cluster/guaranteed-scheduling-critical-addon-pods/
        scheduler.alpha.kubernetes.io/critical-pod: ''
        scheduler.alpha.kubernetes.io/tolerations: >-
          [{"key":"dedicated","operator":"Equal","value":"master","effect":"NoSchedule"}]
    spec:
      serviceAccountName: {{ .ServiceAccount }}
      initContainers:
      - name: install-cni-conf
        image: {{ .Image }}
        command:
          - /bin/sh
          - "-c"
          - "cp -f /etc/cilium-cni/cni-conf.json /host/etc/cni/net.d/{{ .ConfName}}"
        volumeMounts:
        - name: host-cni-conf
          mountPath: /host/etc/cni/net.d
        - name: cilium-cni-config
          mountPath: /etc/cilium-cni/

      - name: install-multus-cni-bin
        image: {{ .MultusImage }}
        command:
          - /bin/sh
          - "-c"
          - "cp -f /usr/bin/multus /host/opt/cni/bin/"
        volumeMounts:
        - name: host-cni-bin
          mountPath: /host/opt/cni/bin/

      - name: install-cni-bin
        image: {{ .Image }}
        command:
          - /bin/sh
          - "-c"
          - "cp -f /usr/lib/cni/* /host/opt/cni/bin/"
        volumeMounts:
        - name: host-cni-bin
          mountPath: /host/opt/cni/bin/

      containers:
      - image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        name: cilium-agent
        command: [ "cilium-agent" ]
        args:
          - "--debug=$(CILIUM_DEBUG)"
          - "--disable-envoy-version-check"
          - "-t=vxlan"
          - "--kvstore=etcd"
          - "--kvstore-opt=etcd.config=/var/lib/etcd-config/etcd.config"
          - "--disable-ipv4=$(DISABLE_IPV4)"
          - "--container-runtime-endpoint={{ .ContainerRuntime }}={{ .ContainerRuntimeSocket }}"
        ports:
          - name: prometheus
            containerPort: 9090
        lifecycle:
          preStop:
            exec:
              command:
                - "rm -f /host/etc/cni/net.d/10-cilium-cni.conf /host/opt/cni/bin/cilium-cni"
        env:
          - name: "K8S_NODE_NAME"
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
          - name: "CILIUM_DEBUG"
            valueFrom:
              configMapKeyRef:
                name: cilium-config
                key: debug
          - name: "DISABLE_IPV4"
            valueFrom:
              configMapKeyRef:
                name: cilium-config
                key: disable-ipv4
        livenessProbe:
          exec:
            command:
            - cilium
            - status
          # The initial delay for the liveness probe is intentionally large to
          # avoid an endless kill & restart cycle if in the event that the initial
          # bootstrapping takes longer than expected.
          initialDelaySeconds: 120
          failureThreshold: 10
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - cilium
            - status
          initialDelaySeconds: 5
          periodSeconds: 5
        volumeMounts:
        {{ if .EnableBPF }}
          - name: bpf-maps
            mountPath: /sys/fs/bpf
        {{end}}
          - name: cilium-run
            mountPath: /var/run/cilium
          - name: host-cni-bin
            mountPath: /host/opt/cni/bin/
          - name: host-cni-conf
            mountPath: /host/etc/cni/net.d
          - name: container-socket
            mountPath: {{ .ContainerRuntimeSocket }}
            readOnly: true
          - name: etcd-config-path
            mountPath: /var/lib/etcd-config
            readOnly: true
          - name: cilium-etcd-secret-mount
            mountPath: /tmp/cilium-etcd
          - name: cilium-cni-config
            mountPath: /etc/cilium-cni/

        securityContext:
          capabilities:
            add:
              - "NET_ADMIN"
          privileged: true
      hostNetwork: true
      volumes:
          # To keep state between restarts / upgrades
        - name: cilium-run
          hostPath:
            path: /var/run/cilium
        {{ if .EnableBPF }}
          # To keep state between restarts / upgrades
        - name: bpf-maps
          hostPath:
            path: /sys/fs/bpf
        {{end}}
          # To read docker events from the node
        - name: container-socket
          hostPath:
            path: {{ .ContainerRuntimeSocket }}
          # To install cilium cni plugin in the host
        - name: host-cni-bin
          hostPath:
            path: {{ .BinDir }}
          # To install cilium cni configuration in the host
        - name: host-cni-conf
          hostPath:
              path: {{ .ConfDir }}
          # To read the etcd config stored in config maps
        - name: etcd-config-path
          configMap:
            name: cilium-config
            items:
            - key: etcd-config
              path: etcd.config
        - name: cilium-etcd-secret-mount
          secret:
            secretName: {{.SecretName}}
        - name: cilium-cni-config
          configMap:
            name: cni-config
            
      restartPolicy: Always
      tolerations:
      - effect: NoSchedule
        key: node-role.kubernetes.io/master
      - effect: NoSchedule
        key: node.cloudprovider.kubernetes.io/uninitialized
        value: "true"
      # Mark cilium's pod as critical for rescheduling
      - key: CriticalAddonsOnly
        operator: "Exists"
      - key: node.kubernetes.io/not-ready
        operator: Exists
        effect: NoSchedule
`
)
