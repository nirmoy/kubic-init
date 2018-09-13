package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
)

const sharedPasswordNamespace = metav1.NamespaceSystem

type SharedPassword struct {
	name     string
	length   int
	contents []byte
}

func NewSharedPassword(name string) SharedPassword {
	return SharedPassword{name: name}
}

func (password *SharedPassword) Rand(length int) ([]byte, error) {
	rawPassword := make([]byte, length)
	if _, err := rand.Read(rawPassword); err != nil {
		return nil, err
	}

	enc := base64.StdEncoding
	encodedPassword := make([]byte, enc.EncodedLen(len(rawPassword)))
	enc.Encode(encodedPassword, rawPassword)

	password.contents = encodedPassword
	return password.contents, nil
}

// String implements the Stringer interface
func (password SharedPassword) String() string {
	return string(password.contents[:])
}

// ToSecret publishes a password as a secret
func (password SharedPassword) ToSecret(client clientset.Interface) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      password.name,
			Namespace: sharedPasswordNamespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			password.name: password.contents,
		},
	}

	if err := apiclient.CreateOrUpdateSecret(client, secret); err != nil {
		return err
	}

	glog.V(3).Infof("[kubic] Created secret %s for password", password.name)
	return nil
}

// FromSecret gets the shared password from a Secret
func (password *SharedPassword) FromSecret(client clientset.Interface) error {
	options := metav1.GetOptions{}
	secret, err := client.CoreV1().Secrets(sharedPasswordNamespace).Get(password.name, options)
	if err != nil {
		return fmt.Errorf("cannot get password from secret %s: %v", password.name, err)
	}

	glog.V(3).Infof("[kubic] Obtained password from secret %s", password.name)
	password.contents = secret.Data[password.name]
	return nil
}
