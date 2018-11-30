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

// Plugin A CNI plugin is a function that is responsible for setting up everything
type Plugin func(*config.KubicInitConfiguration, clientset.Interface) error

type registryMap map[string]Plugin

// Register registers a plugin with the cni
func (registry registryMap) Register(name string, plugin Plugin) {
	registry[name] = plugin
}

// Has checks if it has a registry with the given name
func (registry registryMap) Has(name string) bool {
	_, found := registry[name]
	return found
}

// Load loads a registry
func (registry registryMap) Load(name string, cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	return registry[name](cfg, client)
}

// Registry is the Global Registry
var Registry = registryMap{}
