/*
Copyright 2018 SUSE LINUX GmbH, Nuernberg, Germany..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package loader

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/golang/glog"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
)

// loadFilesIn tries to loads all the files (matching a glob) in a directory,
// returning a list of Buffers
func loadFilesIn(directory string, glob string, descr string) ([]*bytes.Buffer, error) {
	var res = []*bytes.Buffer{}
	glog.V(5).Infof("[kubic] loading %s files from %s", descr, directory)
	files, err := filepath.Glob(filepath.Join(directory, glob))
	if err != nil {
		return nil, err
	}

	glog.V(5).Infof("[kubic] %s files to loadFilesIn: %+v", descr, files)
	for _, f := range files {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %s [%v]", f, err)
		}
		res = append(res, bytes.NewBuffer(b))
	}

	return res, nil
}

// InstallAllAssets tries to install all the assets: CRDs and RBACs
func InstallAllAssets(kubicCfg *kubiccfg.KubicInitConfiguration, manifDir, crdsDir, rbacDir string) error {
	dirs := []string{}

	glog.V(1).Infof("[kubic] installing all the assets...")
	restCfg, err := config.GetConfig()
	kubeadmutil.CheckErr(err)

	if len(manifDir) == 0 {
		manifDir = kubiccfg.DefaultKubicManifestsDir
	}
	dirs = append(kubiccfg.DefaultManifestsDirs, manifDir)
	glog.V(1).Infof("[kubic] looking for manifests in %v", dirs)
	err = InstallManifests(kubicCfg, restCfg, ManifestsInstallOptions{Paths: dirs})
	kubeadmutil.CheckErr(err)

	if len(crdsDir) == 0 {
		crdsDir = kubiccfg.DefaultKubicCRDDir
	}
	dirs = append(kubiccfg.DefaultCRDsDirs, crdsDir)
	glog.V(1).Infof("[kubic] looking for CRDs in %v", dirs)
	err = InstallCRDs(kubicCfg, restCfg, CRDInstallOptions{Paths: dirs})
	if err != nil {
		return err
	}

	if len(rbacDir) == 0 {
		rbacDir = kubiccfg.DefaultKubicRBACDir
	}
	dirs = append(kubiccfg.DefaultRBACDirs, rbacDir)
	glog.V(1).Infof("[kubic] looking for RBACs in %v", dirs)
	err = InstallRBAC(kubicCfg, restCfg, RBACInstallOptions{Paths: dirs})
	if err != nil {
		return err
	}

	return nil
}
