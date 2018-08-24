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
