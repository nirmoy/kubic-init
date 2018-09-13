package autoyast

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/glog"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
	kubicutil "github.com/kubic-project/kubic-init/pkg/util"
)

// the argument that will be used as a password (ie, http://server:8080/?token=XXX.1234)
const secretParameter = "token"

// Content-Type for autoYaST files served
const contentType = "text/xml; charset=UTF-8"

// MakeHandler makes a HTTP handler for serving autoYaST files
func MakeHandler(kubicCfg *kubiccfg.KubicInitConfiguration) (http.HandlerFunc, error) {
	var err error

	glog.V(1).Infof("[kubic] loading AutoYaST template from '%s'", kubicCfg.ClusterFormation.AutoYAST.Template)
	if os.Stat(kubicCfg.ClusterFormation.AutoYAST.Template); err != nil {
		return nil, fmt.Errorf("%q does not exist: %v", kubicCfg.ClusterFormation.AutoYAST.Template, err)
	}

	templateBytes, err := ioutil.ReadFile(kubicCfg.ClusterFormation.AutoYAST.Template)
	if err != nil {
		return nil, fmt.Errorf("unable to read AutoYaST template from %q [%v]", kubicCfg.ClusterFormation.AutoYAST.Template, err)
	}

	sshPublicKey, err := kubiccfg.GetSSHPubicKey()
	if err != nil {
		return nil, fmt.Errorf("unable to read ssh public key: %v", err)
	}

	localIP, err := utilnet.ChooseHostInterface()
	if err != nil {
		return nil, fmt.Errorf("unable to determine local IP address: %v", err)
	}
	kubicCfg.Network.Bind.Address = localIP.String()

	kubicInitContents, err := GetExportedKubicInit(kubicCfg)
	if err != nil {
		return nil, fmt.Errorf("could not generate a exported kubic-init.yaml file: %v", err)
	}

	replacements := struct {
		KubicCfg          *kubiccfg.KubicInitConfiguration
		KubicInitCfgPath  string
		KubicInitContents string
		SSHPublicKey      string
		RegisterEnable    bool
		RegisterCode      string
		RegisterSmtUrl    string
	}{
		kubicCfg,
		kubiccfg.DefaultKubicInitConfigPath,
		kubicInitContents,
		sshPublicKey,
		false, // TODO: where can we get this from?
		"",    // TODO: where can we get this from?
		"",    // TODO: SMT URL
	}

	contents, err := kubicutil.ParseTemplate(string(templateBytes[:]), replacements)
	if err != nil {
		return nil, fmt.Errorf("error when parsing AutoYaST template: %v", err)
	}
	glog.V(8).Infof("[kubic] autoyaml.xml contents:\n%s", contents)

	secret := ""
	if kubicCfg.ClusterFormation.AutoYAST.Protected {
		// TODO: we should try to get the token from the Kubernetes secret...
		if len(kubicCfg.ClusterFormation.Token) == 0 {
			return nil, fmt.Errorf("cannot use AutoYAST protection when no Token is provided")
		}

		secret = kubicCfg.ClusterFormation.Token
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if len(secret) > 0 {
			// check the secret
			secretProvidedLst, provided := r.URL.Query()[secretParameter]
			if !provided || len(secretProvidedLst[0]) < 1 || secretProvidedLst[0] != secret {
				msg := fmt.Sprintf("No token provided with '%s' argument", secretParameter)
				http.Error(w, msg, http.StatusForbidden)
				return
			}
		}

		w.Header().Set("Content-Type", contentType)
		fmt.Fprintf(w, "%s", contents)
	}, nil
}
