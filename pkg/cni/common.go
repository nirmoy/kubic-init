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
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"
	"github.com/renstrom/dedent"

	"github.com/kubic-project/kubic-init/pkg/config"
)

const (
	// DefaultCNIConfName is the default configuration for CNI
	DefaultCNIConfName = "default.conflist"

	// DefaultCNIConflistContents is the default contents in the CNI file
	DefaultCNIConflistContents = `
	{
		"name": "default",
		"cniVersion": "0.3.0",
		"plugins": [
		  {
			"type": "portmap",
			"capabilities": {"portMappings": true}
		  }
		]
	  }
	`
)

// Prepare prepares the CNI deployment
func Prepare(cfg *config.KubicInitConfiguration) error {
	cniConflist := path.Join(config.DefaultCniConfDir, DefaultCNIConfName)

	if len(cfg.Network.Cni.ConfDir) > 0 {
		cniConflist = path.Join(cfg.Network.Cni.ConfDir, DefaultCNIConfName)
	}

	if _, err := os.Stat(cniConflist); os.IsNotExist(err) {
		glog.V(1).Infof("[kubic] creating default configuration for CNI in '%s'", cniConflist)
		contents := []byte(dedent.Dedent(DefaultCNIConflistContents))
		if err := ioutil.WriteFile(cniConflist, contents, 0644); err != nil {
			return err
		}
	} else {
		glog.V(1).Infof("[kubic] default CNI configuration already present at '%s'", cniConflist)
	}

	return nil
}
