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

package loader

import (
	"fmt"
	"os"

	"github.com/kubernetes/kubernetes/cmd/kubeadm/app/util/apiclient"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	kubicutil "github.com/kubic-project/kubic-init/pkg/util"
)

// some globs used for identifying roles, etc...
const (
	roleFileGlob = "*_role.yaml"

	roleBindingFileGlob = "*_role_binding.yaml"
)

const (
	assetsNamespace = metav1.NamespaceSystem
)

// RBACInstallOptions are the options for installing RBACs
type RBACInstallOptions struct {
	// Paths is the path to the directory containing RBACs
	Paths []string

	// ErrorIfPathMissing will cause an error if a Path does not exist
	ErrorIfPathMissing bool
}

// InstallRBAC necessary until https://github.com/kubernetes-sigs/controller-tools/pull/77 is merged
func InstallRBAC(kubicCfg *kubiccfg.KubicInitConfiguration, config *rest.Config, options RBACInstallOptions) error {

	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	restClient := cs.RESTClient()

	for _, path := range kubicutil.RemoveDuplicates(options.Paths) {
		if _, err := os.Stat(path); !options.ErrorIfPathMissing && os.IsNotExist(err) {
			continue
		}

		// load Roles
		roles, err := loadFilesIn(path, roleFileGlob, "role")
		if err != nil {
			return err
		}
		for _, roleBuffer := range roles {
			role := &rbac.ClusterRole{}
			if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), roleBuffer.Bytes(), role); err != nil {
				return fmt.Errorf("unable to decode Role: %v", err)
			}

			role.SetNamespace(assetsNamespace)
			if err = apiclient.CreateOrUpdateClusterRole(cs, role); err != nil {
				return fmt.Errorf("Failed to create new Role: %v", err)
			}
			if err := kubicclient.WaitForObject(restClient, role); err != nil {
				return err
			}
		}

		// load RoleBindings
		roleBindings, err := loadFilesIn(path, roleBindingFileGlob, "role bindings")
		if err != nil {
			return err
		}
		for _, roleBindingsBuffer := range roleBindings {
			roleBinding := &rbac.ClusterRoleBinding{}
			if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), roleBindingsBuffer.Bytes(), roleBinding); err != nil {
				return fmt.Errorf("unable to decode Role bindings: %v", err)
			}

			roleBinding.SetNamespace(assetsNamespace)
			if err = apiclient.CreateOrUpdateClusterRoleBinding(cs, roleBinding); err != nil {
				return fmt.Errorf("Failed to create new Role bindings: %v", err)
			}
			if err := kubicclient.WaitForObject(restClient, roleBinding); err != nil {
				return err
			}
		}
	}

	return nil
}
