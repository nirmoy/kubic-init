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

package multus

const (
	// MultusConfigMapforFlannel the flannel config map
	MultusConfigMapforFlannel = `
kind: ConfigMap
apiVersion: v1
metadata:
  name: cni-config
  namespace: kube-system
  labels:
    tier: node
    app: multus
data:
  cni-conf.json: |
    {
      "name":"multus-cni-network",
      "type":"multus",
      "delegates":[
        {
          "cniVersion":"0.3.1",
          "name":"cbr0",
          "plugins":[
            {
              "type":"flannel",
              "delegate":{
                "forceAddress":true,
                "isDefaultGateway":true
              }
            },
            {
              "type":"portmap",
              "capabilities":{
                "portMappings":true
              }
            }
          ]
        }
      ],
      "kubeconfig" : "{{ .KubeConfig}}"
    }
  net-conf.json: |
    {
      "Network": "{{ .Network }}",
      "Backend":
      {
        "Type": "{{ .Backend }}"
      }
    }
`
	// MultusConfigMapforCilium the cilium config map
	MultusConfigMapforCilium = `
kind: ConfigMap
apiVersion: v1
metadata:
  name: cni-config
  namespace: kube-system
  labels:
    tier: node
    app: multus
data:
  cni-conf.json: |
    {
      "name": "multus-cni-network",
      "type": "multus",
      "delegates": [
        {
          "name": "cilium",
          "type": "cilium-cni"
        }
      ],
      "kubeconfig" : "{{ .KubeConfig}}"
    }
`
)
