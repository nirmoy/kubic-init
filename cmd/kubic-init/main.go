package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1alpha2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha2"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	kubeadmcmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd"
	kubeadmupcmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd/upgrade"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/pkg/version"
)

// The environment variable used for passing the seeder
const seederEnvVar = "SEED_NODE"

// [caas] use a constant set of featureGates
// A set of key=value pairs that describe feature gates for various features.
var defaultFeatureGates = (features.CoreDNS + "=true," +
	features.HighAvailability + "=false," +
	features.SelfHosting + "=true," +
// TODO: disabled until https://github.com/kubernetes/kubeadm/issues/923
	features.StoreCertsInSecrets + "=false," +
// TODO: results in some errors... needs some research
	features.DynamicKubeletConfig + "=false")

// [caas] Hardcoded list of errors to ignore
var defaultIgnoredPreflightErrors = []string{
	"Service-Docker",
	"Swap",
	"FileExisting-crictl",
	"Port-10250",
}

// Version provides the version information of kubic-init
type Version struct {
	ClientVersion *apimachineryversion.Info `json:"clientVersion"`
}

// The kubic-init configuration
type CaasInitConfiguration struct {
	Seeder string
}

// Load a Kubic configuration file, setting some default values
func ConfigFileAndDefaultsToCaasConfig(cfgPath string, internalcfg *CaasInitConfiguration) (*CaasInitConfiguration, error) {
	var err error

	if len(cfgPath) > 0 {
		glog.V(1).Infof("[caas] loading configuration from %s", cfgPath)
		if os.Stat(cfgPath); err != nil {
			return nil, fmt.Errorf("%q does not exist: %v", cfgPath, err)
		}

		b, err := ioutil.ReadFile(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read config from %q [%v]", cfgPath, err)
		}

		decoded, err := kubeadmutil.LoadYAML(b)
		if err != nil {
			return nil, fmt.Errorf("unable to decode config from bytes: %v", err)
		}

		// TODO: check the decoded['kind']

		seeder := decoded["seeder"]
		if seeder != nil && len(seeder.(string)) > 0 {
			if len(internalcfg.Seeder) == 0 {
				internalcfg.Seeder = seeder.(string)
				glog.V(2).Infof("[caas] setting seeder as %s", internalcfg.Seeder)
			}
		}
	}

	// Overwrite some values with environment variables
	if seederEnv, found := os.LookupEnv(seederEnvVar); found {
		internalcfg.Seeder = seederEnv
	}

	return internalcfg, nil
}

// newBootstrapCmd returns a "kubic-init bootstrap" command.
func newBootstrapCmd(out io.Writer) *cobra.Command {
	kubicCfg := &CaasInitConfiguration{}

	masterCfg := &kubeadmapiv1alpha2.MasterConfiguration{}
	kubeadmscheme.Scheme.Default(masterCfg)

	nodeCfg := &kubeadmapiv1alpha2.NodeConfiguration{}
	kubeadmscheme.Scheme.Default(nodeCfg)

	var kubicCfgFile string
	var kubeadmCfgFile string
	var skipTokenPrint = false
	var skipPreFlight = false
	var dryRun = false
	block := true

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the node, either as a seeder or as a regular node depending on the 'seed' config argument.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			kubicCfg, err = ConfigFileAndDefaultsToCaasConfig(kubicCfgFile, kubicCfg)
			kubeadmutil.CheckErr(err)

			featuresGates, err := features.NewFeatureGate(&features.InitFeatureGates, defaultFeatureGates)
			kubeadmutil.CheckErr(err)
			glog.V(3).Infoln("[caas] feature gates: %+v", featuresGates)

			ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(defaultIgnoredPreflightErrors, skipPreFlight)
			kubeadmutil.CheckErr(err)

			if len(kubicCfg.Seeder) > 0 {
				glog.V(1).Infoln("[caas] joining the seeder at %s", kubicCfg.Seeder)
				nodeCfg.DiscoveryTokenAPIServers = []string{kubicCfg.Seeder}
				nodeCfg.FeatureGates = featuresGates

				joiner, err := kubeadmcmd.NewJoin(kubeadmCfgFile, args, nodeCfg, ignorePreflightErrorsSet)
				kubeadmutil.CheckErr(err)

				// TODO: override any nodeCfg parameters we want at this point...

				err = joiner.Run(out)
				kubeadmutil.CheckErr(err)

				glog.V(1).Infoln("[caas] this node should have joined the cluster at this point")

			} else {
				glog.V(1).Infoln("[caas] seeding the cluster from this node")

				masterCfg.FeatureGates = featuresGates

				initter, err := kubeadmcmd.NewInit(kubeadmCfgFile, masterCfg, ignorePreflightErrorsSet, skipTokenPrint, dryRun)
				kubeadmutil.CheckErr(err)

				// TODO: override any masterCfg parameters we want at this point...

				err = initter.Run(out)
				kubeadmutil.CheckErr(err)

				// TODO: create a kubernetes client
				// TODO: deploy CNI, Dex, etc... with this client
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
	flagSet.StringVar(&kubeadmCfgFile, "kubeadm-config", "",
		"Path to kubeadm config file.")
	flagSet.StringVar(&kubicCfg.Seeder, "seeder", "",
		"Cluster seeder.")
	flagSet.BoolVar(&block, "block", block, "Block after boostrapping")
	// Note: All flags that are not bound to the masterCfg object should be whitelisted in cmd/kubeadm/app/apis/kubeadm/validation/validation.go

	return cmd
}

func newCmdVersion(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of kubic-init",
		Run: func(cmd *cobra.Command, args []string) {
			glog.V(1).Infoln("[version] retrieving version info")
			clientVersion := version.Get()
			v := Version{
				ClientVersion: &clientVersion,
			}

			const cflag = "output"
			_, err := cmd.Flags().GetString(cflag)
			if err != nil {
				glog.Fatalf("error accessing flag %s for command %s: %v", cflag, cmd.Name(), err)
			}

			fmt.Fprintf(out, "kubic-init version: %#v\n", v.ClientVersion)
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
