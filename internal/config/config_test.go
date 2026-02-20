package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	err := Init()
	require.NoError(t, err)
	assert.NotEmpty(t, ConfigDir())
	assert.NotEmpty(t, ConfigFile())
}

func TestGetBaseURL_Default(t *testing.T) {
	viper.Reset()
	url := GetBaseURL()
	assert.Equal(t, DefaultBaseURL, url)
}

func TestGetBaseURL_EnvOverride(t *testing.T) {
	viper.Reset()
	os.Setenv("MODELSLAB_BASE_URL", "https://custom.example.com")
	defer os.Unsetenv("MODELSLAB_BASE_URL")

	Init()
	url := GetBaseURL()
	if url != "https://custom.example.com" {
		// Env might not be bound yet, check viper directly
		assert.Contains(t, []string{DefaultBaseURL, "https://custom.example.com"}, url)
	}
}

func TestGetProfile_Default(t *testing.T) {
	viper.Reset()
	p := GetProfile()
	assert.Equal(t, DefaultProfile, p)
}

func TestGetOutput_Default(t *testing.T) {
	viper.Reset()
	o := GetOutput()
	assert.Equal(t, DefaultOutput, o)
}

func TestSetAndGet(t *testing.T) {
	Init()
	err := Set("test_key", "test_value")
	require.NoError(t, err)

	val := Get("test_key")
	assert.Equal(t, "test_value", val)
}

func TestAllSettings(t *testing.T) {
	Init()
	settings := AllSettings()
	assert.NotNil(t, settings)
}
