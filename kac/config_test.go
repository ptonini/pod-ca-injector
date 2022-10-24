package kac

import (
	"context"
	"encoding/base64"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"testing"
)

const (
	configFile      = "../config.yaml"
	testCertificate = `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUIQDtYdVyYzIapvNndRrYojB+STUwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMjEwMjMxOTA4MzVaFw0yMjEx
MjIxOTA4MzVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQC7+i18Z764qsA1abjYvXbO5GSm+xfmaLtjNV+mEf+Z
M7G6cwkGbLERVCeX9sv6eCERCX8loeQP/8WzHsU2npeKeOGhfDsILMexUS7NIRsg
KmzTdv05rfwXAiuUmRjobYf1RIvePV0RO1xv5F7h6ecUDWyhUmtpO+m+T5SSzqRY
AXakZpmhvx8QlKHrS66umDXWm0sR9cYRnAxO6SUtIoLQfYjmFNTpMy6sz3jxezfC
J9YfsEiAS2tH88Q72+kuczQDfz1O3XIKME68ExeWDmRlqeteOPSyGB72LmtBjOgM
zKSfPiD2wzqOfDBAyRhNzg8rNAb9bTMjzKyNOtlK8HAFAgMBAAGjUzBRMB0GA1Ud
DgQWBBT8I7ZHLIlHTN6DvVMCfp1MAVUiqDAfBgNVHSMEGDAWgBT8I7ZHLIlHTN6D
vVMCfp1MAVUiqDAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCL
DZXT82lpcHMwW7lydHn7yx/h++kiYAUZkpOvm6hI2oeQo/OpdgMJIA0KZ/+cbyse
ataa2O6roLmXdv0CnAstVarlggewFPnuVFaBHj4iemH4jvDhpIvJCF62xrSdRpOk
jreVFmb9yvpFHTUInUUcQD/YCSz/dSBJv6ZEK4RpAwPsCwcEya2ijAfc1JpfbIz7
lwArCYp7dk24iqqvHyisEA74KzEIXgec4V443aEnVT0YFrPcCrKTyWO2B1cdg2iF
H9cXQE2NR3ZgAdEWuN5E9M5DA4NufOdEn106iEVloLNlGJ1JA8IyRwn4hRlX9RYa
5qgiE0YULCXL9x9SQDNN
-----END CERTIFICATE-----`
)

var (
	configMap = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"cert.crt": testCertificate,
		},
	}
	secret = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"cert.crt": []byte(base64.StdEncoding.EncodeToString([]byte(testCertificate))),
		},
	}
)

func Test_Config(t *testing.T) {

	ctx := context.Background()
	ctx = context.WithValue(ctx, keyFakeClientSet, true)
	ctx = context.WithValue(ctx, keyFakeObjects, []runtime.Object{configMap, secret})

	_ = os.Setenv("CA_INJECTOR_ANNOTATIONS_INJECT", "ptonini.github.io/inject-ca")
	_ = os.Setenv("CA_INJECTOR_ANNOTATIONS_INJECTED", "ptonini.github.io/ca-injected")
	_ = os.Setenv("CA_INJECTOR_CONFIGMAP_NAME", "ca-injector")

	t.Run("test read config from file", func(t *testing.T) {
		LoadConfig(configFile)
		_, err := getConfig()
		assert.NoError(t, err)
	})

	t.Run("test valid config from configmap", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "configMap", "source": "default/test-config/cert.crt"}}`)
		_ = readConfig("../config.yaml")
		assert.NoError(t, fetchBundles(ctx))
	})

	t.Run("test valid config from secret", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "secret", "source": "default/test-secret/cert.crt"}}`)
		_ = readConfig("../config.yaml")
		assert.NoError(t, fetchBundles(ctx))
	})

	t.Run("test valid config from configmap, no fake client", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "configMap", "source": "default/test-configmap/cert.crt"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(context.WithValue(ctx, keyFakeClientSet, false)))
	})

	t.Run("test valid config from secret, no fake client", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "secret", "source": "default/test-secret/cert.crt"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(context.WithValue(ctx, keyFakeClientSet, false)))
	})

	t.Run("test valid config from nonexistant secret", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "secret", "source": "default/fake/cert.crt"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(ctx))
	})

	t.Run("test valid config from nonexistant secret", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "configMap", "source": "default/fake/cert.crt"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(ctx))
	})

	t.Run("test config with invalid bundle url", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "url", "source": "https://invalid.local"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(ctx))
	})

	t.Run("test config with invalid bundle content", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "url", "source": "https://example.com"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(ctx))
	})

	t.Run("test config with invalid local content", func(t *testing.T) {
		viper.Set("bundles", nil)
		_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"local": {"type": "local", "source": "invalid"}}`)
		_ = readConfig("../config.yaml")
		assert.Error(t, fetchBundles(ctx))
	})

}
