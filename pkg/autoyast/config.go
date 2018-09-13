package autoyast

import (
	"github.com/golang/glog"
	"gopkg.in/yaml.v2"

	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
)

// GetExportedKubicInit generates a kubic-init.yaml file that can be
// used by autoyast'ed nodes
func GetExportedKubicInit(kubicCfg *kubiccfg.KubicInitConfiguration) (string, error) {
	var err error
	var exportedCfg = kubiccfg.KubicInitConfiguration{
		ClusterFormation: kubiccfg.ClusterFormationConfiguration{
			Seeder: kubicCfg.Network.Bind.Address,
			Token: kubicCfg.ClusterFormation.Token,
		},
	}

	marshalled, err := yaml.Marshal(exportedCfg)
	if err != nil {
		return "", err
	}
	glog.V(3).Infof("[kubic] exported kubic-init.yaml:\n%s", marshalled)

	return string(marshalled[:]), nil
}
