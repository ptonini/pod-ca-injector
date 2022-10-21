package kac

import (
	"context"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Config(t *testing.T) {

	ctx := context.Background()

	t.Run("test read invalid file", func(t *testing.T) {
		assert.Error(t, readConfig("../config2.yaml"))
	})

	t.Run("test read valid config", func(t *testing.T) {
		assert.NoError(t, readConfig("../config.yaml"))
	})

	t.Run("test valid config", func(t *testing.T) {
		assert.NoError(t, fetchBundles(ctx))
	})

	t.Run("test config with invalid bundle url", func(t *testing.T) {
		viper.Set("rootCA.baltimore.source", "https://invalid.local")
		assert.Error(t, fetchBundles(ctx))
	})

	t.Run("test config with invalid bundle content", func(t *testing.T) {
		viper.Set("rootCA.baltimore.source", "https://example.com")
		assert.Error(t, fetchBundles(ctx))
	})

	t.Run("test config with invalid local content", func(t *testing.T) {
		viper.Set("rootCA.local.bundle", "invalid")
		assert.Error(t, fetchBundles(ctx))
	})

}
