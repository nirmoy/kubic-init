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

	"github.com/golang/glog"
	"github.com/renstrom/dedent"
)

const (
	// DefaultCNIConflist is the default confinguration for CNI
	DefaultCNIConflist = "/etc/cni/net.d/default.conflist"

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
func Prepare() error {
	if _, err := os.Stat(DefaultCNIConflist); os.IsNotExist(err) {
		glog.V(1).Infof("[kubic] creating default configuration for CNI in '%s'", DefaultCNIConflist)
		contents := []byte(dedent.Dedent(DefaultCNIConflistContents))
		if err := ioutil.WriteFile(DefaultCNIConflist, contents, 0644); err != nil {
			return err
		}
	} else {
		glog.V(1).Infof("[kubic] default CNI configuration already present at '%s'", DefaultCNIConflist)
	}

	return nil
}
