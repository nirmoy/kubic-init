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
	kubeadmoptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	kubeadmupcmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd/upgrade"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	"k8s.io/kubernetes/pkg/version"

	kubicboot "github.com/kubic-project/kubic-init/pkg/bootstrap"
)

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

	return internalcfg, nil
}

// newBootstrapCmd returns a "kubic-init bootstrap" command.
func newBootstrapCmd(out io.Writer) *cobra.Command {
	kubicCfg := &CaasInitConfiguration{}

	externalMasterCfg := &kubeadmapiv1alpha2.MasterConfiguration{}
	kubeadmscheme.Scheme.Default(externalMasterCfg)

	nodeCfg := &kubeadmapiv1alpha2.NodeConfiguration{}
	kubeadmscheme.Scheme.Default(nodeCfg)

	var kubicCfgFile string
	var kubeadmCfgFile string
	block := true

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the node, either as a seeder or as a regular node depending on the 'seed' config argument.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			kubicCfg, err = ConfigFileAndDefaultsToCaasConfig(kubicCfgFile, kubicCfg)
			kubeadmutil.CheckErr(err)

			featureGates, err := features.NewFeatureGate(&features.InitFeatureGates, defaultFeatureGates)
			kubeadmutil.CheckErr(err)

			ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(defaultIgnoredPreflightErrors, false)
			kubeadmutil.CheckErr(err)

			if len(kubicCfg.Seeder) > 0 {

				glog.V(1).Infoln("[caas] joining the seeder at %s", kubicCfg.Seeder)

				// TODO: convert the "seeder" to a discovery-thing

				nodeCfg, err := configutil.NodeConfigFileAndDefaultsToInternalConfig(kubeadmCfgFile, nodeCfg)
				kubeadmutil.CheckErr(err)

				// set some args and do some checks
				nodeCfg.FeatureGates = featureGates
				nodeCfg.DiscoveryTokenAPIServers = args
				if nodeCfg.NodeRegistration.Name == "" {
					glog.V(1).Infoln("[join] found NodeName empty")
					glog.V(1).Infoln("[join] considered OS hostname as NodeName")
				}

				err = kubicboot.Join(nodeCfg, ignorePreflightErrorsSet)
				kubeadmutil.CheckErr(err)

				glog.V(1).Infoln("[caas] this node should have joined the cluster at this point")
			} else {
				glog.V(1).Infoln("[caas] seeding the cluster from this node")

				// Either use the config file if specified, or convert the defaults in the external to an internal externalMasterCfg representation
				cfg, err := configutil.ConfigFileAndDefaultsToInternalConfig(kubeadmCfgFile, externalMasterCfg)
				kubeadmutil.CheckErr(err)

				cfg.FeatureGates = featureGates

				// Create the options object for the bootstrap token-related flags, and override the default value for .Description
				bto := kubeadmoptions.NewBootstrapTokenOptions()
				bto.Description = "The default bootstrap token generated by 'kubeadm init'."

				err = kubicboot.Init(externalMasterCfg, cfg, bto, ignorePreflightErrorsSet)
				kubeadmutil.CheckErr(err)

				// TODO: load CNI, Dex, etc...

				glog.V(1).Infoln("[caas] the seeder is ready at this point")
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
		"Path to kubic config file.")
	flagSet.StringVar(&kubeadmCfgFile, "kubeadm-config", "",
		"Path to kubeadm config file. WARNING: Usage of a configuration file is experimental.")
	flagSet.StringVar(&kubicCfg.Seeder, "seeder", "",
		"Cluster seeder.")
	flagSet.BoolVar(&block, "block", block, "Block after boostrapping")
	// Note: All flags that are not bound to the externalMasterCfg object should be whitelisted in cmd/kubeadm/app/apis/kubeadm/validation/validation.go

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

			const flag = "output"
			_, err := cmd.Flags().GetString(flag)
			if err != nil {
				glog.Fatalf("error accessing flag %s for command %s: %v", flag, cmd.Name(), err)
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
