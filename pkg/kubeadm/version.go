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

package kubeadm

import (
	"encoding/json"

	"github.com/golang/glog"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd"

	"github.com/kubic-project/kubic-init/pkg/config"
)

// NewVersion gets the kubeadm version information
func NewVersion(kubicCfg *config.KubicInitConfiguration, args ...string) (cmd.Version, error) {
	args = append([]string{
		"--output=json",
	}, args...)

	output, err := kubeadmCmdOut("version", kubicCfg, args...)
	if err != nil {
		return cmd.Version{}, err
	}
	glog.V(8).Infof("[kubic] `kubeadm` version output: %s", output.String())

	v := cmd.Version{}
	dec := json.NewDecoder(&output)
	if err := dec.Decode(&v); err != nil {
		return cmd.Version{}, err
	}

	return v, nil
}
