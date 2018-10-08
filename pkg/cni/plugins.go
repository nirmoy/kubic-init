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

package cni

import (
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kubic-project/kubic-init/pkg/config"
)

// A CNI plugin is a function that is responsible for setting up everything
type CniPlugin func(*config.KubicInitConfiguration, clientset.Interface) error

type CniRegistry map[string]CniPlugin

func (registry CniRegistry) Register(name string, plugin CniPlugin) {
	registry[name] = plugin
}

func (registry CniRegistry) Has(name string) bool {
	_, found := registry[name]
	return found
}

func (registry CniRegistry) Load(name string, cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	return registry[name](cfg, client)
}

// Global Registry
var Registry = CniRegistry{}
