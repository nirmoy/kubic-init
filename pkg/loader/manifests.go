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
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	"github.com/kubic-project/kubic-init/pkg/util"
)

const (
	yamlFileGlob = "*.yaml"

	urlFileGlob = "*.url"
)

// ManifestsInstallOptions are the options for installing manifests
type ManifestsInstallOptions struct {
	// Paths is the path to the directory containing manifests
	Paths []string

	// ErrorIfPathMissing will cause an error if a Path does not exist
	ErrorIfPathMissing bool
}

// getUnstructuredInYAMLFile gets a list of objects in a YAML file
func getUnstructuredInYAMLFile(kubicCfg *kubiccfg.KubicInitConfiguration, fileContents string) []*unstructured.Unstructured {
	res := []*unstructured.Unstructured{}

	sepYamlfiles := strings.Split(fileContents, "---")
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

		// we cannot create an "unstructured" directly from YAML: we must convert
		// it first to JSON...
		fJSON, err := yaml.ToJSON([]byte(fReplaced))
		if err != nil {
			glog.V(1).Infof("[kubic] ERROR: when converting to JSON: %v", err)
			continue
		}

		us := &unstructured.Unstructured{}
		err = us.UnmarshalJSON(fJSON)
		if err != nil {
			glog.V(1).Infof("[kubic] ERROR: when unmarshalling JSON: %v", err)
			continue
		}
		res = append(res, us)
	}

	return res
}

// getUnstructuredFromURL gets a list of objects in a remote YAML file found in a URL
func getUnstructuredFromURL(kubicCfg *kubiccfg.KubicInitConfiguration, url string) []*unstructured.Unstructured {
	url = strings.TrimSpace(url)
	glog.V(3).Infof("[kubic] getting manifest from '%s'", url)
	client := http.Client{
		Timeout: time.Duration(30 * time.Second),
	}

	resp, err := client.Get(url)
	if err != nil {
		glog.V(3).Infof("[kubic] ERROR: while reading manifest from '%s': %s", url, err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.V(3).Infof("[kubic] ERROR: while reading manifest from '%s': %s", url, err)
		return nil
	}

	glog.V(8).Infof("[kubic] manifest obtained from '%s': %s", url, body)
	return getUnstructuredInYAMLFile(kubicCfg, string(body))
}

// InstallManifests installs all the manifests found in the manifests directory
// It will do a best-effort job, ignoring errors
func InstallManifests(kubicCfg *kubiccfg.KubicInitConfiguration, config *rest.Config, options ManifestsInstallOptions) error {
	for _, path := range util.RemoveDuplicates(options.Paths) {
		if _, err := os.Stat(path); !options.ErrorIfPathMissing && os.IsNotExist(err) {
			continue
		}

		// process all the local manifests
		filesBuffers, err := loadFilesIn(path, yamlFileGlob, "manifest")
		if err != nil {
			return err
		}
		for _, fileBuffer := range filesBuffers {
			for _, unstr := range getUnstructuredInYAMLFile(kubicCfg, fileBuffer.String()) {
				err = kubicclient.CreateOrUpdateFromUnstructured(config, unstr)
				if err != nil {
					glog.V(3).Infof("[kubic] ERROR: could not load manifest: ignored")
				}
			}
		}

		// process all the remote manifests
		urlsBuffers, err := loadFilesIn(path, urlFileGlob, "URLs")
		if err != nil {
			return err
		}
		for _, urlBuffer := range urlsBuffers {
			for _, unstr := range getUnstructuredFromURL(kubicCfg, urlBuffer.String()) {
				err = kubicclient.CreateOrUpdateFromUnstructured(config, unstr)
				if err != nil {
					glog.V(3).Infof("[kubic] ERROR: could not load manifest: ignored")
				}
			}
		}
	}

	return nil
}
