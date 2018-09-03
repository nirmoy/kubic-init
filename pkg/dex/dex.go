package dex

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"path/filepath"

	"github.com/golang/glog"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	certutil "k8s.io/client-go/util/cert"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"

	"github.com/kubic-project/kubic-init/pkg/config"
	"github.com/kubic-project/kubic-init/pkg/util"
)

const (
	// The namespace where Dex will be run
	DexNamespace = metav1.NamespaceSystem

	// DexClusterRoleName sets the name for the dex ClusterRole
	DexClusterRoleName = "suse:kubic:dex"

	DexClusterRoleNameRead = "suse:kubic:read-dex-service"

	DexClusterRoleNameLDAP = "suse:kubic:ldap-administrators"

	DexClusterRoleNamePSP = "suse:kubic:psp:dex"

	// TODO: maybe configurable in "kubic-init.yaml"...
	DexLDAPAdminGroupName = "Administrators"

	// DexServiceAccountName describes the name of the ServiceAccount for the dex addon
	DexServiceAccountName = "dex"

	DexCertsBasename = "dex"

	// TODO: currently this must match the XXX in XXX.{key,crt}
	DexCertsSecretName = "dex"

	// The image to use for Dex
	DexImage = "registry.opensuse.org/devel/caasp/kubic-container/container/kubic/caasp-dex:2.7.1"

	DexHealthPort = 8471
)

// shared passwords
var (
	// we will need some shared passwords for connecting with Velum, Kubernetes, etc
	DexSharedPasswordsSecrets = []string{
		"dex-kubernetes",
		"dex-velum",
		"dex-cli",
	}

	// the length (in bytes) for these passwords
	DexSharedPasswordsLength = 16
)

var (
	serviceAccount = v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DexServiceAccountName,
			Namespace: DexNamespace,
			Labels: map[string]string{
				"kubernetes.io/cluster-service": "true",
			},
		},
	}

	clusterRole = &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: DexClusterRoleName,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"dex.coreos.com"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"create"},
			},
		},
	}

	clusterRoleRead = &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DexClusterRoleNameRead,
			Namespace: DexNamespace,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"services"},
				ResourceNames: []string{DexServiceAccountName},
				Verbs:         []string{"get"},
			},
		},
	}

	clusterRoleBinding = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DexClusterRoleName,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     DexClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      DexServiceAccountName,
				Namespace: DexNamespace,
			},
		},
	}

	clusterRoleBindingLDAP = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DexClusterRoleNameLDAP,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     config.DefaultClusterAdminRole,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.GroupKind,
				Name:      DexLDAPAdminGroupName,
				Namespace: DexNamespace,
			},
		},
	}

	clusterRoleBindingPSP = rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DexClusterRoleNamePSP,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "suse:kubic:psp:privileged",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      DexServiceAccountName,
				Namespace: DexNamespace,
			},
		},
	}
)

// Load creates the dex addon
func Load(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	var err error

	if len(cfg.Services.Dex.LDAP) == 0 {
		glog.Infoln("[kubic] Dex not loaded: no LDAP services have been specified in the configuration.")
		return nil
	}

	if err := createServiceAccount(client); err != nil {
		return fmt.Errorf("error when creating dex service account: %v", err)
	}
	cert, _, err := createCertificates(cfg, client)
	if err != nil {
		return err
	}
	sharedPasswords, err := createSharedPasswords()
	if err != nil {
		return err
	}
	if err := sharedPasswordsToSecrets(sharedPasswords, client); err != nil {
		return err
	}
	configMap, err := createConfigMap(cfg, sharedPasswords)
	if err != nil {
		return err
	}
	deployment, err := createDeployment(cfg, configMap, cert)
	if err != nil {
		return err
	}
	if err := createDexAddon(configMap, deployment, client); err != nil {
		return err
	}
	if err := createRBACRules(cfg, client); err != nil {
		return fmt.Errorf("error when creating dex RBAC rules: %v", err)
	}
	fmt.Println()
	glog.V(1).Infoln("[kubic] applied essential addon: dex")
	return nil
}

func createConfigMap(cfg *config.KubicInitConfiguration, sharedPasswords map[string]util.SharedPassword) ([]byte, error) {
	var err error

	glog.V(3).Infoln("[kubic] generating ConfigMap for Dex")

	// get a valid address for the "issuer" in the Dex configuration
	dexAddress, err := cfg.GetPublicAPIAddress()
	if err != nil {
		return nil, err
	}

	replacements := struct {
		KubicCfg           *config.KubicInitConfiguration
		DexNamespace       string
		DexAddress         string
		DexPort            int
		DexSharedPasswords map[string]util.SharedPassword
		CaCrt              string
	}{
		cfg,
		DexNamespace,
		dexAddress,
		cfg.Services.Dex.NodePort,
		sharedPasswords,
		filepath.Join(cfg.Certificates.Directory, kubeadmconstants.CACertName),
	}


	finalConfigMap, err := util.ParseTemplate(DexConfigMap, replacements)
	if err != nil {
		return nil, fmt.Errorf("error when parsing Dex configmap template: %v", err)
	}
	glog.V(8).Infoln("[kubic] ConfigMap for Dex:\n%s\n")
	return []byte(finalConfigMap), nil
}

// createSharedPasswords creates all the shared passwords necessary
func createSharedPasswords() (map[string]util.SharedPassword, error) {
	sharedPasswords := map[string]util.SharedPassword{}

	glog.V(8).Infoln("[kubic] creating shared passwords for Dex")
	for _, spName := range DexSharedPasswordsSecrets {
		sharedPassword := util.NewSharedPassword(spName)
		if _, err := sharedPassword.Rand(DexSharedPasswordsLength); err != nil {
			return nil, err
		}
		sharedPasswords[spName] = sharedPassword
	}
	return sharedPasswords, nil
}

// createSharedPasswords publishes all the shared passwords as Secrets in the apiserver
func sharedPasswordsToSecrets(sharedPasswords map[string]util.SharedPassword, client clientset.Interface) error {
	glog.V(8).Infoln("[kubic] publishing Dex shared passwords as secrets")
	for _, sharedPassword := range sharedPasswords {
		if err := sharedPassword.ToSecret(client); err != nil {
			return err
		}
	}
	return nil
}

// createDeployment performs replacements in the Deployment
func createDeployment(cfg *config.KubicInitConfiguration, configMap []byte, cert *x509.Certificate) ([]byte, error) {
	var err error

	glog.V(3).Infoln("[kubic] generating deployment for Dex")

	// calculate the sha256 of the configmap
	configMapSha := sha256.Sum256(configMap)

	// ... and the certificate, so any change on one of these would results in
	// kubernetes restarting the Dex containers
	encodedCert := certutil.EncodeCertPEM(cert)
	certSha := sha256.Sum256(encodedCert)

	replacements := struct {
		KubicCfg           *config.KubicInitConfiguration
		DexImage           string
		DexServiceAccount  string
		DexNamespace       string
		DexCertsSecretName string
		DexConfigMapSha    string
		DexCertSha         string
		CaCrtPath          string
	}{
		cfg,
		DexImage,
		DexServiceAccountName,
		DexNamespace,
		DexCertsSecretName,
		fmt.Sprintf("%x", configMapSha),
		fmt.Sprintf("%x", certSha),
		filepath.Join(cfg.Certificates.Directory, kubeadmconstants.CACertName),
	}

	finalDeployment, err := util.ParseTemplate(DexDeployment, replacements)
	if err != nil {
		return nil, fmt.Errorf("error when parsing Dex deployment template: %v", err)
	}
	glog.V(8).Infoln("[kubic] Dex deployment:\n%s\n", finalDeployment)

	return []byte(finalDeployment), nil
}

func createDexAddon(configMapBytes, deploymentBytes []byte, client clientset.Interface) error {
	dexConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configMapBytes, dexConfigMap); err != nil {
		return fmt.Errorf("unable to decode dex configmap %v", err)
	}

	// Create the ConfigMap for dex or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, dexConfigMap); err != nil {
		return err
	}

	dexDeployment := &apps.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), deploymentBytes, dexDeployment); err != nil {
		return fmt.Errorf("unable to decode dex daemonset %v", err)
	}

	// Create the Deployment for dex or update it in case it already exists
	return apiclient.CreateOrUpdateDeployment(client, dexDeployment)
}

// createCertificates creates creates certificates for Dex in /etc/kubernetes/pki
// and uploads them to the API server as secrets
func createCertificates(cfg *config.KubicInitConfiguration, client clientset.Interface) (*x509.Certificate, *rsa.PrivateKey, error) {
	certsCfg := certutil.Config{
		CommonName:   DexCertsBasename,
		Organization: []string{kubeadmconstants.MastersGroup},
		// TODO: check we need both key usages
		Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	cert, key, err := util.CreateAndUploadCertificates(cfg.Certificates.Directory, client, DexCertsBasename, certsCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while creating Dex key and certificate: %v", err)
	}

	return cert, key, nil
}

// createRBACRules creates the essential RBAC rules for a minimally set-up cluster
func createRBACRules(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	glog.V(3).Infoln("[kubic] creating RBAC rules for Dex")

	if err := createClusterRoles(client); err != nil {
		return err
	}

	if err := createClusterRoleBindings(cfg, client); err != nil {
		return err
	}

	return nil
}

// createClusterRoles creates a cluster role
func createClusterRoles(client clientset.Interface) error {
	var err error

	// Try to replicate the old behaviour we had in:
	// https://github.com/kubic-project/salt/blob/master/salt/addons/dex/manifests/05-clusterrole.yaml

	glog.V(3).Infoln("[kubic] creating ClusterRole %s", DexClusterRoleName)
	err = apiclient.CreateOrUpdateClusterRole(client, clusterRole)
	if err != nil {
		return err
	}

	// Try to replicate the old behaviour we had in:
	// https://github.com/kubic-project/salt/blob/master/salt/addons/dex/manifests/05-clusterrole.yaml
	glog.V(3).Infoln("[kubic] creating ClusterRole %s", DexClusterRoleNameRead)
	err = apiclient.CreateOrUpdateClusterRole(client, clusterRoleRead)
	if err != nil {
		return err
	}

	return nil
}

// createServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func createServiceAccount(client clientset.Interface) error {
	// Try to replicate the old behaviour we had in:
	// https://github.com/kubic-project/salt/blob/master/salt/addons/dex/manifests/05-serviceaccount.yaml
	glog.V(3).Infoln("[kubic] creating serviceAccount %s", DexServiceAccountName)
	return apiclient.CreateOrUpdateServiceAccount(client, &serviceAccount)
}

// createClusterRoleBindings creates all the bindings
func createClusterRoleBindings(cfg *config.KubicInitConfiguration, client clientset.Interface) error {
	// Try to replicate the old behaviour we had in:
	// https://github.com/kubic-project/salt/blob/master/salt/addons/dex/manifests/10-clusterrolebinding.yaml

	// Map the Dex SA to the Kubernetes cluster-admin role
	err := apiclient.CreateOrUpdateClusterRoleBinding(client, &clusterRoleBinding)
	if err != nil {
		return err
	}

	// Map the Dex SA to the Kubernetes cluster-admin role
	err = apiclient.CreateOrUpdateClusterRoleBinding(client, &clusterRoleBindingLDAP)
	if err != nil {
		return err
	}

	if cfg.Features.PSP {
		err = apiclient.CreateOrUpdateClusterRoleBinding(client, &clusterRoleBindingPSP)
		if err != nil {
			return err
		}
	}

	return nil
}
