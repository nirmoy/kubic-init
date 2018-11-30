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

package crypto

import (
	"math/rand"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes/kubernetes/cmd/kubeadm/app/util/apiclient"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kubic-project/kubic-init/pkg/util"
)

var (
	sharedPasswordNamespace = metav1.NamespaceSystem

	// the length (in bytes) for these passwords
	sharedPasswordDefaultLen = 16
)

// SharedPassword struct
type SharedPassword struct {
	Name     string // The name includes the "namespace" (ie, "kube-system/dex-velum")
	length   int
	contents string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randStringRunes(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// NewSharedPassword returns a new SharedPassword struct
func NewSharedPassword(name, namespace string) SharedPassword {
	if len(namespace) == 0 {
		namespace = sharedPasswordNamespace
	}
	return SharedPassword{
		Name: util.NamespacedNameToString(util.NewNamespacedName(name, namespace)),
	}
}

// Rand returns a new password of langth
func (password *SharedPassword) Rand(length int) (string, error) {
	if length == 0 {
		length = sharedPasswordDefaultLen
	}
	password.contents = randStringRunes(length)
	return password.contents, nil
}

// GetName returns the name
func (password SharedPassword) GetName() string {
	return util.StringToNamespacedName(password.Name).Name
}

// GetNamespace returns the namespace
func (password SharedPassword) GetNamespace() string {
	return util.StringToNamespacedName(password.Name).Namespace
}

// String implements the Stringer interface
func (password SharedPassword) String() string {
	return string(password.contents[:])
}

// CreateOrUpdateToSecret publishes a password as a secret
func (password SharedPassword) CreateOrUpdateToSecret(cli clientset.Interface) error {
	secret := &corev1.Secret{
		ObjectMeta: util.NamaspacedObjToMeta(password),
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			password.GetName(): []byte(password.contents),
		},
	}
	if err := apiclient.CreateOrUpdateSecret(cli, secret); err != nil {
		return err
	}
	glog.V(3).Infof("[kubic] created Secret %s for password", password.GetName())
	return nil
}

// GetFromSecret gets the shared password from a Secret
func (password *SharedPassword) GetFromSecret(cli clientset.Interface) error {
	found, err := cli.CoreV1().Secrets(password.GetNamespace()).Get(password.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	glog.V(3).Infof("[kubic] there is an existing password for '%s'", password.GetName())
	password.contents = string(found.Data[password.GetName()])
	return nil
}

// AsSecretReference returns the SharedPassword in the form of a corev1.SecretReference
func (password *SharedPassword) AsSecretReference() corev1.SecretReference {
	return corev1.SecretReference{
		Name:      password.GetName(),
		Namespace: password.GetNamespace(),
	}
}

// Delete deletes
func (password *SharedPassword) Delete(cli clientset.Interface) error {
	err := cli.CoreV1().Secrets(password.GetNamespace()).Delete(password.GetName(), &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// ////////////////////////////////////////////////////////////////////////////////////////

// SharedPasswordsSet is a groups of static, shared passwords that can be saved
// to k8s Secrets.
type SharedPasswordsSet map[string]SharedPassword

// NewSharedPasswordsSet creates all the shared passwords
// it tries to load those passwords from Secrets in the apiserver
// if they are not found, new random passwords are generated,
// but not persisted in the apiserver
func NewSharedPasswordsSet(cli clientset.Interface, names []string, namespace string) (SharedPasswordsSet, error) {
	sharedPasswords := SharedPasswordsSet{}

	// by default, passwords are stored in the "kube-system" namespace
	if len(namespace) == 0 {
		namespace = sharedPasswordNamespace
	}

	glog.V(8).Infof("[kubic] creating/getting %d shared passwords", len(names))
	for _, name := range names {
		glog.V(8).Infof("[kubic] generating/getting shared password '%s'", name)
		sharedPassword := NewSharedPassword(name, namespace)
		if err := sharedPassword.GetFromSecret(cli); apierrors.IsNotFound(err) {
			glog.V(8).Infof("[kubic] shared password '%s' not found: generating random value", name)
			if _, err := sharedPassword.Rand(sharedPasswordDefaultLen); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		sharedPasswords[name] = sharedPassword
	}

	return sharedPasswords, nil
}

// CreateOrUpdateToSecrets publishes all the shared passwords as Secrets in the apiserver
func (sharedPasswords SharedPasswordsSet) CreateOrUpdateToSecrets(cli clientset.Interface) error {
	glog.V(8).Infof("[kubic] publishing %d shared passwords as Secrets", len(sharedPasswords))
	for _, sharedPassword := range sharedPasswords {
		if err := sharedPassword.CreateOrUpdateToSecret(cli); err != nil {
			return err
		}
	}
	return nil
}
