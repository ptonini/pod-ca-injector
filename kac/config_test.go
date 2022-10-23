package kac

import (
	"context"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

const configFile = "../config.yaml"

func Test_Config(t *testing.T) {

	ctx := context.Background()

	_ = os.Setenv("CA_INJECTOR_ANNOTATIONS_INJECT", "ptonini.github.io/inject-ca")
	_ = os.Setenv("CA_INJECTOR_ANNOTATIONS_INJECTED", "ptonini.github.io/ca-injected")
	_ = os.Setenv("CA_INJECTOR_CONFIGMAP_NAME", "ca-injector")
	_ = os.Setenv("CA_INJECTOR_ROOTCA", `{"remote": {"type": "url", "source": "https://www.digicert.com/CACerts/BaltimoreCyberTrustRoot.crt.pem"}, "local": {"type": "local", "source": "-----BEGIN CERTIFICATE-----\nYYYYY\n-----END CERTIFICATE-----"}}`)

	t.Run("test read valid config", func(t *testing.T) {
		LoadConfig(configFile)
		StartConfigWatch(configFile)
		_, err := getConfig()
		assert.NoError(t, err)
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
