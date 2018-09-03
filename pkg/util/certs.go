package util

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/certs/pkiutil"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
)

// CreateAndUploadCertificates creates certificates for an cluster service, signed with the ca.{crt|key}
// found in the local /etc/kuberentes/pki directory, and uploads them to the API server as secrets.
func CreateAndUploadCertificates(pkiDir string, client clientset.Interface, name string, certCfg certutil.Config) (*x509.Certificate, *rsa.PrivateKey, error) {
	caCrt, caKey, err := loadCertificateAuthority(pkiDir, constants.CACertAndKeyBaseName)
	if err != nil {
		return nil, nil, err
	}

	// create a certificate, signed by the CA
	cert, key, err := pkiutil.NewCertAndKey(caCrt, caKey, certCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while creating %s key and certificate: %v", name, err)
	}

	cert, err = writeCertificateFilesIfNotExist(pkiDir, name, cert, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while writing %s key and certificate: %v", name, err)
	}

	if err = uploadTLSSecrets(pkiDir, client, name); err != nil {
		return nil, nil, fmt.Errorf("failure while writing %s key and certificate: %v", name, err)
	}
	return cert, key, nil
}

// loadCertificateAuthority loads certificate authority
func loadCertificateAuthority(pkiDir string, baseName string) (*x509.Certificate, *rsa.PrivateKey, error) {
	glog.V(3).Infof("[kubic] Loading '%s' CA from %s", baseName, pkiDir)

	// Checks if certificate authority exists in the PKI directory
	if !pkiutil.CertOrKeyExist(pkiDir, baseName) {
		return nil, nil, fmt.Errorf("couldn't load %s certificate authority from %s", baseName, pkiDir)
	}

	// Try to load certificate authority .crt and .key from the PKI directory
	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, baseName)
	if err != nil {
		return nil, nil, fmt.Errorf("failure loading %s certificate authority: %v", baseName, err)
	}

	// Make sure the loaded CA cert actually is a CA
	if !caCert.IsCA {
		return nil, nil, fmt.Errorf("%s certificate is not a certificate authority", baseName)
	}

	return caCert, caKey, nil
}

// writeCertificateFilesIfNotExist write a new certificate to the given path.
// If there already is a certificate file at the given path it tries to load it and check if the values in the
// existing and the expected certificate equals. If they do; kubeadm will just skip writing the file as it's up-to-date,
// otherwise this function returns an error.
func writeCertificateFilesIfNotExist(pkiDir string, baseName string, cert *x509.Certificate, key *rsa.PrivateKey) (*x509.Certificate, error) {
	var err error

	// If cert or key exists, we should try to load them
	if pkiutil.CertOrKeyExist(pkiDir, baseName) {

		// Try to load .crt and .key from the PKI directory
		cert, _, err = pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, baseName)
		if err != nil {
			return nil, fmt.Errorf("failure loading %s certificate: %v", baseName, err)
		}

		glog.V(3).Infof("[kubic] Using the existing %s certificate and key.\n", baseName)
	} else {

		// Write .crt and .key files to disk
		if err := pkiutil.WriteCertAndKey(pkiDir, baseName, cert, key); err != nil {
			return nil, fmt.Errorf("failure while saving %s certificate and key: %v", baseName, err)
		}

		glog.V(3).Infof("[kubic] Generated %s certificate and key.\n", baseName)
	}

	return cert, nil
}

// uploadTLSSecrets loads the certificate for `name` and uploads the crt|key as a TLS secret
func uploadTLSSecrets(pkiDir string, client clientset.Interface, name string) error {
	cert, key, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, name)
	if err != nil {
		return err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceSystem,
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			v1.TLSCertKey:       certutil.EncodeCertPEM(cert),
			v1.TLSPrivateKeyKey: certutil.EncodePrivateKeyPEM(key),
		},
	}

	if err := apiclient.CreateOrUpdateSecret(client, secret); err != nil {
		return err
	}
	glog.V(3).Infof("[kubic] Created TLS secret %q: from %s.crt and %s.key\n", name, name, name)

	return nil
}
