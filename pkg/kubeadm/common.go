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
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"

	"github.com/kubic-project/kubic-init/pkg/config"
)

const kubeadmConfigTemplate = "kubic-kubeadm.*.yaml"

type stringConsumer func(msg string)

// kubeadm runs a "kubeadm" command
func kubeadm(name string, kubicCfg *config.KubicInitConfiguration,
	stdoutProc stringConsumer, stderrProc stringConsumer,
	args ...string) error {

	args = append([]string{name}, args...)

	kubeadmPath := kubicCfg.Paths.Kubeadm

	// Now we can run the "kubeadm" command
	glog.V(1).Infof("[kubic] exec: %s %s", kubeadmPath, strings.Join(args, " "))
	cmd := exec.Command(kubeadmPath, args...)

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	stdoutScan := bufio.NewScanner(stdoutPipe)
	stderrScan := bufio.NewScanner(stderrPipe)

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		for stdoutScan.Scan() {
			stdoutProc(stdoutScan.Text())
		}
	}()

	go func() {
		for stderrScan.Scan() {
			stderrProc(stderrScan.Text())
		}
	}()

	return cmd.Wait()
}

// toKubeadmConfig is a function that can translate a kubic-init config to
// a kubeadm config
type toKubeadmConfig func(*config.KubicInitConfiguration, map[string]bool) ([]byte, error)

// kubeadmWithConfig runs a "kubeadm" command
func kubeadmWithConfig(name string, kubicCfg *config.KubicInitConfiguration, configer toKubeadmConfig, args ...string) error {

	featureGates, err := features.NewFeatureGate(&features.InitFeatureGates, config.DefaultFeatureGates)
	kubeadmutil.CheckErr(err)
	glog.V(3).Infof("[kubic] feature gates: %+v", featureGates)

	// generate a kubeadm config file
	// some kubeadm commands do not really need any configuration, so this is optional
	if configer != nil {
		configFile, err := ioutil.TempFile(os.TempDir(), kubeadmConfigTemplate)
		if err != nil {
			return err
		}
		defer os.Remove(configFile.Name())

		// get the configuration
		marshalledBytes, err := configer(kubicCfg, featureGates)
		if err != nil {
			return err
		}
		if glog.V(3) {
			glog.Infoln("[kubic] kubeadm configuration produced:")
			for _, line := range strings.Split(string(marshalledBytes), "\n") {
				glog.Infof("[kubic]         %s", line)
			}
		}

		// ... and write them in a file
		configFile.Write(marshalledBytes)

		args = append(args, "--config="+configFile.Name())
	}

	printer := func(m string) {
		fmt.Println(m)
	}
	logger := func(m string) {
		glog.V(1).Infoln(m)
	}

	return kubeadm(name, kubicCfg, logger, printer, args...)
}

// kubeadmCmdOut runs a "kubeadm" command waiting for the
func kubeadmCmdOut(name string, kubicCfg *config.KubicInitConfiguration, args ...string) (bytes.Buffer, error) {

	output := bytes.Buffer{}
	bufferer := func(m string) {
		output.WriteString(m)
	}

	if err := kubeadm(name, kubicCfg, bufferer, bufferer, args...); err != nil {
		return bytes.Buffer{}, err
	}
	return output, nil
}

// getIgnorePreflightArg returns the arg for ignoring pre-flight errors
func getIgnorePreflightArg() string {
	ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(config.DefaultIgnoredPreflightErrors)
	if err != nil {
		panic(err)
	}

	arg := "--ignore-preflight-errors=" + strings.Join(ignorePreflightErrorsSet.List(), ",")
	return arg
}

func getVerboseArg() string {
	return "--v=3" // TODO: make this configurable
}
