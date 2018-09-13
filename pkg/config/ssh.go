package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/glog"
)

var DefaultSSHKeyFiles = []string{
	"${HOME}/.ssh/id_dsa.pub",
	"${HOME}/.ssh/id_rsa.pub",
}

// GetSSHPubicKey tries to obtain the SSH public key
func GetSSHPubicKey() (string, error) {
	var err error

	for _, key := range DefaultSSHKeyFiles {
		key = os.ExpandEnv(key)

		glog.V(3).Infof("[kubic] Checking we can get the SSH public key from '%s'", key)
		if os.Stat(key); err == nil {
			contents, err := ioutil.ReadFile(key)
			if err != nil {
				return "", fmt.Errorf("unable to read public key file from %q [%v]", key, err)
			}

			s := strings.TrimSpace(string(contents[:]))
			glog.V(3).Infof("[kubic] Found SSH public key at '%s'", key)
			return s, nil
		}
	}

	return "", nil
}
