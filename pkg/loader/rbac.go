package loader

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes/kubernetes/cmd/kubeadm/app/util/apiclient"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	kubicclient "github.com/kubic-project/kubic-init/pkg/client"
)

// some globs used for identifying roles, etc...
const (
	roleFileGlob = "*_role.yaml"

	roleBindingFileGlob = "*_role_binding.yaml"
)

const (
	assetsNamespace = metav1.NamespaceSystem
)

// RBACInstallOptions are the options for installing CRDs
type RBACInstallOptions struct {
	// Paths is the path to the directory containing CRDs
	Paths []string

	// ErrorIfPathMissing will cause an error if a Path does not exist
	ErrorIfPathMissing bool

	// maxTime is the max time to wait
	maxTime time.Duration

	// pollInterval is the interval to check
	pollInterval time.Duration
}

// loadFilesIn tries to loads all the files (matching a glob) in a directory,
// returning a list of Buffers
func loadFilesIn(directory string, glob string, descr string) ([]*bytes.Buffer, error) {
	var res = []*bytes.Buffer{}
	glog.V(5).Infof("[kubic] loading %s files from %s", descr, directory)
	files, err := filepath.Glob(filepath.Join(directory, glob))
	if err != nil {
		return nil, err
	}

	glog.V(5).Infof("[kubic] %s files to loadFilesIn: %+v", descr, files)
	for _, f := range files {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %s [%v]", f, err)
		}
		res = append(res, bytes.NewBuffer(b))
	}

	return res, nil
}

// necessary until https://github.com/kubernetes-sigs/controller-tools/pull/77 is merged
func InstallRBAC(config *rest.Config, options RBACInstallOptions) error {

	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	restClient := cs.RESTClient()

	for _, path := range options.Paths {
		if _, err := os.Stat(path); !options.ErrorIfPathMissing && os.IsNotExist(err) {
			glog.V(3).Infof("[kubic] WARNING directory %s does not exist: cannot load files from there.", path)
			continue
		}

		// load Roles
		roles, err := loadFilesIn(path, roleFileGlob, "role")
		if err != nil {
			return err
		}
		for _, roleBuffer := range roles {
			role := &rbac.Role{}
			if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), roleBuffer.Bytes(), role); err != nil {
				return fmt.Errorf("unable to decode Role: %v", err)
			}

			role.SetNamespace(assetsNamespace)
			if err = apiclient.CreateOrUpdateRole(cs, role); err != nil {
				return fmt.Errorf("Failed to create new Role: %v", err)
			}
			if err := kubicclient.WaitForObject(restClient, role); err != nil {
				return err
			}
		}

		// load RoleBindings
		roleBindings, err := loadFilesIn(path, roleBindingFileGlob, "role bindings")
		if err != nil {
			return err
		}
		for _, roleBindingsBuffer := range roleBindings {
			roleBinding := &rbac.RoleBinding{}
			if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), roleBindingsBuffer.Bytes(), roleBinding); err != nil {
				return fmt.Errorf("unable to decode Role bindings: %v", err)
			}

			roleBinding.SetNamespace(assetsNamespace)
			if err = apiclient.CreateOrUpdateRoleBinding(cs, roleBinding); err != nil {
				return fmt.Errorf("Failed to create new Role bindings: %v", err)
			}
			if err := kubicclient.WaitForObject(restClient, roleBinding); err != nil {
				return err
			}
		}
	}

	return nil
}
