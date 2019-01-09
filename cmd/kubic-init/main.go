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
	goversion "github.com/hashicorp/go-version"
	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
	kubiccluster "github.com/kubic-project/kubic-init/pkg/cluster"
	"github.com/kubic-project/kubic-init/pkg/cni"
	_ "github.com/kubic-project/kubic-init/pkg/cni/cilium"
	_ "github.com/kubic-project/kubic-init/pkg/cni/flannel"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	"github.com/kubic-project/kubic-init/pkg/kubeadm"
	"github.com/kubic-project/kubic-init/pkg/loader"
)

// Version to be set from the build process
var Version string

// Build to be set by the build process
var Build string

// BuildDate to be set by the build process
var BuildDate string

// Branch to be set by the build process
var Branch string

// GoVersion to be set by the build process
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
	needsBootstrap := true
	needsReset := false

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

			// get the kubeadm version
			kubeadmVersionInfo, err := kubeadm.NewVersion(kubicCfg)
			kubeadmutil.CheckErr(err)
			kubeadmVersion := fmt.Sprintf("%s.%s", kubeadmVersionInfo.ClientVersion.Major, kubeadmVersionInfo.ClientVersion.Minor)
			glog.V(1).Infof("[kubic] kubeadm version: %s", kubeadmVersion)

			// .. and check it is ok for the kubernetes version we are trying to manage
			parsedKubeadmVersion, err := goversion.NewVersion(kubeadmVersion)
			kubeadmutil.CheckErr(err)
			parsedKubernetesVersion, err := goversion.NewVersion(kubiccfg.DefaultKubernetesVersion)
			kubeadmutil.CheckErr(err)
			if parsedKubeadmVersion.LessThan(parsedKubernetesVersion) {
				glog.V(1).Infof("[kubic] FATAL: invalid kubeadm version: %s when we want to create a %s cluster",
					kubeadmVersion, kubiccfg.DefaultKubernetesVersion)
			} else {
				glog.V(1).Infof("[kubic] kubeadm version looks right")
			}

			// check if there is a valid admin.conf in this machine
			// in that case, try to connect to that API server and
			// check if this node is already registered. if that is the case,
			// no bootstrap is needed, and if something goes wrong, a reset
			// is necessary...
			// TODO: we should check if the kubic-init.yaml has changed:
			// TODO: that would mean we would need to reset things
			adminKubeconfig := kubeadmconstants.GetAdminKubeConfigPath()
			_, err = os.Stat(adminKubeconfig)
			if err != nil {
				if os.IsNotExist(err) {
					if kubiccfg.KubeadmLeftovers() {
						glog.V(1).Infoln("[kubic] no existing 'admin.conf' but some leftovers found: we will reset the environment")
						needsReset = true
					} else {
						glog.V(1).Infoln("[kubic] no existing 'admin.conf' found: assuming this is the first setup")
					}
				} else {
					// some error, but not the admin-conf-doesn't-exist error
					glog.V(1).Infof("[kubic] error when looking for an existing admin.conf: %s", err)
					needsReset = true
				}
			} else {
				glog.V(1).Infoln("[kubic] there is an existing kubeconfig: checking wether the node is already set up...")

				// there is already a `/etc/kubernetes/admin.conf` file: check if
				// the node is already setup and registered
				client, err := kubeconfigutil.ClientSetFromFile(adminKubeconfig)
				if err != nil {
					needsReset = true
				} else {
					// gets the name of the current node
					nodeName, err := kubiccfg.GetNodeNameFromKubeletConfig()
					if err != nil {
						glog.V(1).Infoln("[kubic] could not get the node name: will reset the environmet")
						needsReset = true
					} else {
						glog.V(1).Infof("[kubic] node name: %s", nodeName)

						// gets the corresponding node and retrives attributes stored there.
						node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
						if err != nil {
							glog.V(1).Infof("[kubic] could not get registration info for %s: will reset the environment", nodeName)
							needsReset = true
						} else {
							kubiccfg.PrintNodeProperties(node)
							needsReset = false
							needsBootstrap = false
						}
					}
				}
			}

			if needsReset {
				glog.V(1).Infoln("[kubic] resetting environment before bootstrapping")
				err = kubeadm.NewReset(kubicCfg)
				kubeadmutil.CheckErr(err)
			}

			if needsBootstrap {
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
						glog.V(1).Infof("[kubic] preparing CNI")
						err = cni.Prepare(kubicCfg)
						kubeadmutil.CheckErr(err)

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
			} else {
				glog.V(1).Infoln("[kubic] node was already setup: no bootstrap is necessary")
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
	flagSet.BoolVar(&needsReset, "reset", needsReset, "force a reset before starting the bootstrap")

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
