package dex

import (
	"testing"

	certutil "k8s.io/client-go/util/cert"

	"github.com/kubic-project/kubic-init/pkg/config"
)

func TestCreateDexConfigMap(t *testing.T) {

	// The kubic-init configuration
	cfg := config.KubicInitConfiguration{
		Services: config.ServicesConfiguration{
			Dex: config.DexConfiguration{
				NodePort: 32000,
				LDAP: []config.DexLDAPConfiguration{
					{
						Name:   "test",
						BindDN: "some-bind-dn",
						BindPW: "some-bind-pwd",
						User: config.DexLDAPUserConfiguration{
							BaseDN: "some-base",
							Filter: "some-filter",
							AttrMap: map[string]string{
								"username": "some-username",
								"id":       "some-id",
								"email":    "the@email.com",
								"name":     "some-name",
							},
						},
					},
				},
			},
		},
	}

	// create all the shared passwords we need
	sharedPasswords, err := createSharedPasswords()
	if err != nil {
		t.Fatalf("Could not create a shared passwords for tests: %s", err)
	}

	configMap, err := createConfigMap(&cfg, sharedPasswords)
	if err != nil {
		t.Fatalf("Could not generate ConfigMap for Dex: %s", err)
	}

	configMapStr := string(configMap[:])
	t.Logf("ConfigMap generated for Dex:\n%s\n", configMapStr)

	// TODO: perform more sophisticated checks...
}

func TestCreateDexDeployment(t *testing.T) {

	// The kubic-init configuration
	cfg := config.KubicInitConfiguration{
		Services: config.ServicesConfiguration{
			Dex: config.DexConfiguration{
				NodePort: 32000,
				LDAP: []config.DexLDAPConfiguration{
					{
						Name:   "test",
						BindDN: "some-bind-dn",
						BindPW: "some-bind-pwd",
						User: config.DexLDAPUserConfiguration{
							BaseDN: "some-base",
							Filter: "some-filter",
							AttrMap: map[string]string{
								"username": "some-username",
								"id":       "some-id",
								"email":    "the@email.com",
								"name":     "some-name",
							},
						},
					},
				},
			},
		},
	}

	// create some fake certificate (just for getting the SHA256)
	key, err := certutil.NewPrivateKey()
	if err != nil {
		t.Fatalf("Could not create a certificate for tests: %s", err)
	}

	certCfg := certutil.Config{}
	cert, err := certutil.NewSelfSignedCACert(certCfg, key)
	if err != nil {
		t.Fatalf("Could not create a certificate for tests: %s", err)
	}

	// create all the shared passwords we need
	sharedPasswords, err := createSharedPasswords()
	if err != nil {
		t.Fatalf("Could not create a shared passwords for tests: %s", err)
	}

	// create the configmap
	configMap, err := createConfigMap(&cfg, sharedPasswords)
	if err != nil {
		t.Fatalf("Could not generate configMap for Dex: %s", err)
	}

	// and finally create the deployment
	deployment, err := createDeployment(&cfg, configMap, cert)
	if err != nil {
		t.Fatalf("Could not generate Deployment  for Dex: %s", err)
	}

	deploymentStr := string(deployment[:])
	t.Logf("Deployment generated for Dex:\n%s\n", deploymentStr)

	// TODO: perform more sophisticated checks...
}
