package controller

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	rbac "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// some globs used for identifying CRDs, roles, etc...
const (
	crdFileGlob = "*.yaml"

	roleFileGlob = "*_role.yaml"

	roleBindingFileGlob = "*_role_binding.yaml"
)

const (
	assetsNamespace = metav1.NamespaceSystem
)

// load loads all the files (matching a glob) in a directory, returning a list of Buffers
func load(directory string, glob string, descr string) ([]*bytes.Buffer, error) {
	var res = []*bytes.Buffer{}
	glog.V(5).Infof("[kubic] loading %s files from %s", descr, directory)
	files, err := filepath.Glob(filepath.Join(directory, glob))
	if err != nil {
		return nil, err
	}

	glog.V(5).Infof("[kubic] %s files to load: %+v", descr, files)
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
func LoadAssets(client client.Client, apiextensionsclient *apiextensionsclientset.Clientset, crdsDir, rbacDir string) error {
	ctx := context.TODO()

	// load CRDs
	crds, err := load(crdsDir, crdFileGlob, "CRD")
	if err != nil {
		return err
	}

	apiExtRESTClient := apiextensionsclient.RESTClient()

	for _, crdBuffers := range crds {
		crd := &apiextensionsv1beta1.CustomResourceDefinition{}
		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), crdBuffers.Bytes(), crd); err != nil {
			return fmt.Errorf("unable to decode CRD: %v", err)
		}

		crd.ObjectMeta.SetNamespace(assetsNamespace)

		glog.V(5).Infof("[kubic] creating CRD '%s/%s'", crd.GetNamespace(), crd.GetName())
		_, err := apiextensionsclient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.Name, metav1.GetOptions{})
		existing, err := apiextensionsclient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.Name, metav1.GetOptions{})
		if kubeerr.IsNotFound(err) {
			glog.V(5).Infof("[kubic] %s: creating", err)
			_, err = apiextensionsclient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			// it seems we cannot just update the CRD: we must take the "existing" one,
			// update the Spec, and then update() on the "existing" CRD
			existing.Spec.Validation = crd.Spec.Validation
			_, err = apiextensionsclient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(existing)
			if err != nil {
				return err
			}
		}

		request := apiExtRESTClient.Get().AbsPath("apis", crd.Spec.Group, crd.Spec.Version, crd.Spec.Names.Plural)
		if err := waitForURL(request); err != nil {
			return err
		}
		glog.V(5).Infof("[kubic] CRD %s/%s created/updated successfuly", crd.GetNamespace(), crd.GetName())
	}

	// load Roles
	roles, err := load(rbacDir, roleFileGlob, "role")
	if err != nil {
		return err
	}
	for _, roleBuffer := range roles {
		role := &rbac.Role{}
		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), roleBuffer.Bytes(), role); err != nil {
			return fmt.Errorf("unable to decode Role: %v", err)
		}

		role.SetNamespace(assetsNamespace)
		glog.V(5).Infof("[kubic] creating Role '%s/%s'", role.GetNamespace(), role.GetName())
		if err = client.Create(ctx, role); err != nil {
			if !kubeerr.IsAlreadyExists(err) {
				return fmt.Errorf("Failed to create new Role: %v", err)
			}

			glog.V(5).Infof("[kubic] updating existing Role '%s/%s'", role.GetNamespace(), role.GetName())
			if err := client.Update(ctx, role); err != nil {
				return fmt.Errorf("Failed to update Role %s: %v", role.GetName(), err)
			}
			glog.V(5).Infof("[kubic] Role '%s/%s' updated successfully", role.GetNamespace(), role.GetName())
		} else {
			glog.V(5).Infof("[kubic] Role '%s/%s' created successfully", role.GetNamespace(), role.GetName())
		}

		request := apiExtRESTClient.Get().AbsPath(role.GetSelfLink())
		if err := waitForURL(request); err != nil {
			return err
		}
	}

	// load RoleBindings
	roleBindings, err := load(rbacDir, roleBindingFileGlob, "role bindings")
	if err != nil {
		return err
	}
	for _, roleBindingsBuffer := range roleBindings {
		roleBindings := &rbac.RoleBinding{}
		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), roleBindingsBuffer.Bytes(), roleBindings); err != nil {
			return fmt.Errorf("unable to decode Role bindings: %v", err)
		}

		roleBindings.SetNamespace(assetsNamespace)
		glog.V(5).Infof("[kubic] creating Role bindings '%s/%s'", roleBindings.GetNamespace(), roleBindings.GetName())
		if err = client.Create(ctx, roleBindings); err != nil {
			if !kubeerr.IsAlreadyExists(err) {
				return fmt.Errorf("Failed to create new Role bindings: %v", err)
			}

			glog.V(5).Infof("[kubic] updating existing Role bindings '%s/%s'", roleBindings.GetNamespace(), roleBindings.GetName())
			if err := client.Update(ctx, roleBindings); err != nil {
				return fmt.Errorf("Failed to update Role bindings %s: %v", roleBindings.GetName(), err)
			}
			glog.V(5).Infof("[kubic] Role bindings '%s/%s' updated successfully", roleBindings.GetNamespace(), roleBindings.GetName())
		} else {
			glog.V(5).Infof("[kubic] Role bindings '%s/%s' created successfully", roleBindings.GetNamespace(), roleBindings.GetName())
		}

		request := apiExtRESTClient.Get().AbsPath(roleBindings.GetSelfLink())
		if err := waitForURL(request); err != nil {
			return err
		}
	}

	return nil
}

func waitForURL(request *rest.Request) error {
	glog.V(5).Infof("[kubic] Waiting until URL is ready...")
	err := wait.Poll(2*time.Second, 5*time.Minute, func() (bool, error) {
		res := request.Do()
		err := res.Error()
		if err != nil {
			// RESTClient returns *apierrors.StatusError for any status codes < 200 or > 206
			// and http.Client.Do errors are returned directly.
			if se, ok := err.(*kubeerr.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}

		var statusCode int
		res.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			return false, fmt.Errorf("invalid status code: %d", statusCode)
		}

		return true, nil
	})

	return err
}
