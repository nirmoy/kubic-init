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
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1alpha2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha2"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	kubeadmcmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd"
	kubeadmupcmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd/upgrade"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	"github.com/kubic-project/kubic-init/pkg/cni"
	_ "github.com/kubic-project/kubic-init/pkg/cni/flannel"
	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
)

// to be set from the build process
var Version string
var Build string

// newBootstrapCmd returns a "kubic-init bootstrap" command.
func newBootstrapCmd(out io.Writer) *cobra.Command {
	kubicCfg := &kubiccfg.KubicInitConfiguration{}

	masterCfg := &kubeadmapiv1alpha2.MasterConfiguration{}
	kubeadmscheme.Scheme.Default(masterCfg)

	nodeCfg := &kubeadmapiv1alpha2.NodeConfiguration{}
	kubeadmscheme.Scheme.Default(nodeCfg)

	var kubicCfgFile string
	var skipTokenPrint = false
	var skipPreFlight = false
	var dryRun = false
	block := true

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the node, either as a seeder or as a regular node depending on the 'seed' config argument.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			kubicCfg, err = kubiccfg.ConfigFileAndDefaultsToKubicInitConfig(kubicCfgFile)
			kubeadmutil.CheckErr(err)

			featuresGates, err := features.NewFeatureGate(&features.InitFeatureGates, kubiccfg.DefaultFeatureGates)
			kubeadmutil.CheckErr(err)
			glog.V(3).Infof("[caas] feature gates: %+v", featuresGates)

			ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(kubiccfg.DefaultIgnoredPreflightErrors, skipPreFlight)
			kubeadmutil.CheckErr(err)

			if len(kubicCfg.ClusterFormation.Seeder) > 0 {
				glog.V(1).Infoln("[caas] joining the seeder at %s", kubicCfg.ClusterFormation.Seeder)

				nodeCfg.FeatureGates = featuresGates

				err = kubiccfg.KubicInitConfigToNodeConfig(kubicCfg, nodeCfg)
				kubeadmutil.CheckErr(err)

				joiner, err := kubeadmcmd.NewJoin("", args, nodeCfg, ignorePreflightErrorsSet)
				kubeadmutil.CheckErr(err)

				err = joiner.Run(out)
				kubeadmutil.CheckErr(err)

				glog.V(1).Infoln("[caas] this node should have joined the cluster at this point")

			} else {
				glog.V(1).Infoln("[caas] seeding the cluster from this node")

				masterCfg.FeatureGates = featuresGates

				err = kubiccfg.KubicInitConfigToMasterConfig(kubicCfg, masterCfg)
				kubeadmutil.CheckErr(err)

				initter, err := kubeadmcmd.NewInit("", masterCfg, ignorePreflightErrorsSet, skipTokenPrint, dryRun)
				kubeadmutil.CheckErr(err)

				err = initter.Run(out)
				kubeadmutil.CheckErr(err)

				// create a kubernetes client
				// create a connection to the API server and wait for it to come up
				client, err := kubeconfigutil.ClientSetFromFile(kubeadmconstants.GetAdminKubeConfigPath())
				kubeadmutil.CheckErr(err)

				glog.V(1).Infof("[caas] deploying CNI DaemonSet with '%s' driver", kubicCfg.Cni.Driver)
				err = cni.Registry.Load(kubicCfg.Cni.Driver, kubicCfg, client)
				kubeadmutil.CheckErr(err)

				// TODO: deploy Dex, etc...
			}

			if block {
				glog.V(1).Infoln("[caas] control plane ready... looping forever")
				for {
					time.Sleep(time.Second)
				}
			}
		},
	}

	flagSet := cmd.PersistentFlags()
	flagSet.StringVar(&kubicCfgFile, "config", "",
		"Path to kubic-init config file.")
	flagSet.BoolVar(&block, "block", block, "Block after boostrapping")
	// Note: All flags that are not bound to the masterCfg object should be whitelisted in cmd/kubeadm/app/apis/kubeadm/validation/validation.go

	return cmd
}

func newCmdVersion(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of kubic-init",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "kubic-init version: %s (build: %s)", Version, Build)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format; available options are 'yaml', 'json' and 'short'")
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
	cmds.AddCommand(newBootstrapCmd(os.Stdout))
	cmds.AddCommand(kubeadmcmd.NewCmdReset(os.Stdin, os.Stdout))
	cmds.AddCommand(kubeadmupcmd.NewCmdUpgrade(os.Stdout))
	cmds.AddCommand(newCmdVersion(os.Stdout))

	err := cmds.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
