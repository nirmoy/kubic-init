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
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
	kubiccluster "github.com/kubic-project/kubic-init/pkg/cluster"
	"github.com/kubic-project/kubic-init/pkg/cni"
	_ "github.com/kubic-project/kubic-init/pkg/cni/flannel"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	"github.com/kubic-project/kubic-init/pkg/kubeadm"
	"github.com/kubic-project/kubic-init/pkg/loader"
)

// to be set from the build process
var Version string
var Build string
var BuildDate string
var Branch string
var GoVersion string

// newCmdBootstrap returns a "kubic-init bootstrap" command.
func newCmdBootstrap(out io.Writer) *cobra.Command {
	kubicCfg := &kubiccfg.KubicInitConfiguration{}

	var kubicCfgFile string
	var vars = []string{}

	var postControlManifDir = kubiccfg.DefaultKubicManifestsDir
	var crdsDir = kubiccfg.DefaultKubicCRDDir
	var rbacDir = kubiccfg.DefaultKubicRBACDir

	loadAssets := true
	block := true
	deployCNI := true

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the node, either as a seeder or as a regular node depending on the 'seed' config argument.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			glog.V(1).Infof("[kubic] version: %s", Version)
			glog.V(1).Infof("[kubic] build:   %s", Build)
			glog.V(1).Infof("[kubic] date:    %s", BuildDate)
			glog.V(1).Infof("[kubic] branch:  %s", Branch)
			glog.V(1).Infof("[kubic] go:      %s", GoVersion)

			kubicCfg, err = kubiccfg.FileAndDefaultsToKubicInitConfig(kubicCfgFile)
			kubeadmutil.CheckErr(err)

			err = kubicCfg.SetVars(vars)
			kubeadmutil.CheckErr(err)

			if !kubicCfg.IsSeeder() {
				glog.V(1).Infof("[kubic] joining the seeder at %s", kubicCfg.ClusterFormation.Seeder)
				err := kubeadm.NewJoin(kubicCfg)
				kubeadmutil.CheckErr(err)
				glog.V(1).Infoln("[kubic] this node should have joined the cluster at this point")
			} else {
				glog.V(1).Infoln("[kubic] seeding the cluster from this node")
				err := kubeadm.NewInit(kubicCfg)
				kubeadmutil.CheckErr(err)

				// create a kubernetes client
				// create a connection to the API server and wait for it to come up
				client, err := kubeconfigutil.ClientSetFromFile(kubeadmconstants.GetAdminKubeConfigPath())
				kubeadmutil.CheckErr(err)

				// upload the seeder configuration to a ConfigMap
				extraLabels := map[string]string{
					"kubic-seeder-version": fmt.Sprintf("%s", Version),
					"kubic-seeder-build":   fmt.Sprintf("%s", Build),
				}
				err = kubicCfg.ToConfigMap(client, kubiccfg.DefaultKubicInitConfigmap, extraLabels)
				kubeadmutil.CheckErr(err)

				if !kubicCfg.ClusterFormation.AutoApprove {
					glog.V(1).Infoln("[kubic] removing the auto-approval rules for new nodes")
					err = kubiccluster.RemoveAutoApprovalRBAC(client)
					kubeadmutil.CheckErr(err)
				} else {
					glog.V(1).Infoln("[kubic] new nodes will be accepted automatically")
				}

				if deployCNI {
					glog.V(1).Infof("[kubic] deploying CNI DaemonSet with '%s' driver", kubicCfg.Network.Cni.Driver)
					err = cni.Registry.Load(kubicCfg.Network.Cni.Driver, kubicCfg, client)
					kubeadmutil.CheckErr(err)
				} else {
					glog.V(1).Infof("[kubic] WARNING: CNI will not be deployed")
				}

				if loadAssets {
					kubeconfig, err := kubicclient.GetConfig()
					kubeadmutil.CheckErr(err)

					glog.V(1).Infof("[kubic] trying to load assets...")
					err = loader.InstallAllAssets(kubeconfig, kubicCfg, postControlManifDir, crdsDir, rbacDir)
					kubeadmutil.CheckErr(err)
				} else {
					glog.V(1).Infof("[kubic] WARNING: not trying to load assets")
				}
			}

			if block {
				glog.V(1).Infoln("[kubic] control plane ready... looping forever")
				for {
					time.Sleep(time.Second)
				}
			}
		},
	}

	flagSet := cmd.PersistentFlags()
	flagSet.StringVar(&kubicCfgFile, "config", "", "path to kubic-init config file.")
	flagSet.BoolVar(&block, "block", block, "block after boostrapping")
	flagSet.StringSliceVar(&vars, "var", []string{}, "set a configuration variable (ie, Network.Cni.Driver=cilium")
	flagSet.BoolVar(&deployCNI, "deploy-cni", deployCNI, "deploy the CNI driver")

	// assets
	flagSet.BoolVar(&loadAssets, "load-assets", loadAssets, "load the CRDs, RBACs and manifests")
	flagSet.StringVar(&crdsDir, "crds-dir", crdsDir, "load CRDs from this directory.")
	flagSet.StringVar(&rbacDir, "rbac-dir", rbacDir, "load RBACs from this directory.")
	flagSet.StringVar(&postControlManifDir, "manif-dir", postControlManifDir, "load manifests from this directory.")

	return cmd
}

// newCmdReset returns the "kubic-init reset" command
func newCmdReset(in io.Reader, out io.Writer) *cobra.Command {
	kubicCfg := &kubiccfg.KubicInitConfiguration{}

	var kubicCfgFile string
	var vars = []string{}

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Run this to revert any changes made to this host by kubic-init.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			kubicCfg, err = kubiccfg.FileAndDefaultsToKubicInitConfig(kubicCfgFile)
			kubeadmutil.CheckErr(err)

			err = kubicCfg.SetVars(vars)
			kubeadmutil.CheckErr(err)

			err = kubeadm.NewReset(kubicCfg)
			kubeadmutil.CheckErr(err)

			// TODO: perform any kubic-specific cleanups here
		},
	}

	flagSet := cmd.PersistentFlags()
	flagSet.StringVar(&kubicCfgFile, "config", "", "Path to kubic-init config file.")
	flagSet.StringSliceVar(&vars, "var", []string{}, "Set a configuration variable (ie, Network.Cni.Driver=cilium")

	return cmd
}

func newCmdVersion(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of kubic-init",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "kubic-init: version: %s\n", Version)
			fmt.Fprintf(out, "            build:   %s\n", Build)
			fmt.Fprintf(out, "            date:    %s\n", BuildDate)
			fmt.Fprintf(out, "            branch:  %s\n", Branch)
			fmt.Fprintf(out, "            go:      %s\n", GoVersion)
		},
	}
	return cmd
}

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// see https://github.com/kubernetes/kubernetes/issues/17162#issuecomment-225596212
	flag.CommandLine.Parse([]string{})

	pflag.Set("logtostderr", "true")

	cmds := &cobra.Command{
		Use:   "kubic-init",
		Short: "kubic-init: easily bootstrap a secure Kubernetes cluster",
		Long: dedent.Dedent(`
			kubic-init: easily bootstrap a secure Kubernetes cluster.
		`),
	}

	cmds.ResetFlags()
	cmds.AddCommand(newCmdBootstrap(os.Stdout))
	cmds.AddCommand(newCmdReset(os.Stdin, os.Stdout))
	cmds.AddCommand(newCmdVersion(os.Stdout))

	err := cmds.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
