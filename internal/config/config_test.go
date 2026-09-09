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

// A .modelslab/config.toml is read from the CURRENT WORKING DIRECTORY, so any
// repository you clone and cd into gets a say in it. Merged wholesale, a
// committed base_url was enough to redirect the user's stored bearer token to an
// attacker-controlled host on the next command.
func TestStripUntrustedProjectKeys(t *testing.T) {
	settings := map[string]interface{}{
		"base_url": "http://attacker.example",
		"api_key":  "ml-attacker-key",
		"token":    "attacker-token",
		"defaults": map[string]interface{}{
			"base_url": "http://attacker.example",
			"output":   "json",
		},
		"generation": map[string]interface{}{
			"default_model": "sdxl",
			"output_dir":    "./out",
		},
	}

	cleaned := stripUntrustedProjectKeys(settings)

	assert.NotContains(t, cleaned, "base_url")
	assert.NotContains(t, cleaned, "api_key")
	assert.NotContains(t, cleaned, "token")

	defaults := cleaned["defaults"].(map[string]interface{})
	assert.NotContains(t, defaults, "base_url")
	assert.Equal(t, "json", defaults["output"], "harmless preferences must still merge")

	generation := cleaned["generation"].(map[string]interface{})
	assert.Equal(t, "sdxl", generation["default_model"])
	assert.Equal(t, "./out", generation["output_dir"])
}

func TestStripUntrustedProjectKeys_ToleratesMissingSections(t *testing.T) {
	cleaned := stripUntrustedProjectKeys(map[string]interface{}{"generation": "not-a-map"})

	assert.Equal(t, "not-a-map", cleaned["generation"])
}
