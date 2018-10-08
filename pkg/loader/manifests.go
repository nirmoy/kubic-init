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
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes/kubernetes/cmd/kubeadm/app/util/apiclient"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	"github.com/kubic-project/kubic-init/pkg/util"
)

const (
	yamlFileGlob = "*.yaml"
)

// ManifestsInstallOptions are the options for installing manifests
type ManifestsInstallOptions struct {
	// Paths is the path to the directory containing manifests
	Paths []string

	// ErrorIfPathMissing will cause an error if a Path does not exist
	ErrorIfPathMissing bool
}

// getObjectsInYAMLFile gets a list of objects in a YAML file
func getObjectsInYAMLFile(kubicCfg *kubiccfg.KubicInitConfiguration, fileContents string) []runtime.Object {
	sepYamlfiles := strings.Split(fileContents, "---")
	res := make([]runtime.Object, 0, len(sepYamlfiles))
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		replacements := struct {
			KubicCfg *kubiccfg.KubicInitConfiguration
		}{
			kubicCfg,
		}

		fReplaced, err := util.ParseTemplate(f, replacements)
		if err != nil {
			glog.V(1).Infof("[kubic] ERROR: when parsing manifest template: %v", err)
			continue
		}
		glog.V(8).Infof("[kubic] Dex deployment:\n%s\n", fReplaced)

		decode := clientsetscheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(fReplaced), nil, nil)
		if err != nil {
			glog.V(3).Infof("[kubic] ERROR: while decoding YAML object: %s", err)
			continue
		}

		res = append(res, obj)
	}
	return res
}

func InstallManifests(kubicCfg *kubiccfg.KubicInitConfiguration, config *rest.Config, options ManifestsInstallOptions) error {
	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	restClient := cs.RESTClient()

	for _, path := range util.RemoveDumplicates(options.Paths) {
		if _, err := os.Stat(path); !options.ErrorIfPathMissing && os.IsNotExist(err) {
			continue
		}

		// load Roles
		filesBuffers, err := loadFilesIn(path, yamlFileGlob, "manifest")
		if err != nil {
			return err
		}

		for _, fileBuffer := range filesBuffers {
			for _, object := range getObjectsInYAMLFile(kubicCfg, fileBuffer.String()) {
				gvk := object.GetObjectKind().GroupVersionKind()
				glog.V(3).Infof("[kubic] Loading %s found...", gvk.String())

				switch o := object.(type) {
				case *corev1.Pod:
					if _, err = kubicclient.CreateOrUpdatePod(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Pod: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *appsv1.Deployment:
					if err = apiclient.CreateOrUpdateDeployment(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Deployment: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *appsv1.DaemonSet:
					if err = apiclient.CreateOrUpdateDaemonSet(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update DaemonSet: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *batchv1.Job:
					if _, err = kubicclient.CreateOrUpdateJob(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Job: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *rbacv1.Role:
					if err = apiclient.CreateOrUpdateRole(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Role: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *rbacv1.RoleBinding:
					if err = apiclient.CreateOrUpdateRoleBinding(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Role bindings: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *rbacv1.ClusterRole:
					if err = apiclient.CreateOrUpdateClusterRole(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Cluster Role: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *rbacv1.ClusterRoleBinding:
					if err = apiclient.CreateOrUpdateClusterRoleBinding(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Cluster Role bindings: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				case *corev1.ServiceAccount:
					if err = apiclient.CreateOrUpdateServiceAccount(cs, o); err != nil {
						return fmt.Errorf("Failed to create/update Service Account: %v", err)
					}
					if err := kubicclient.WaitForObject(restClient, o); err != nil {
						return err
					}

				default:
					glog.V(3).Infof("[kubic] WARNING: unsupported class %s.", gvk.String())
					continue
				}
			}
		}
	}
	return nil
}
