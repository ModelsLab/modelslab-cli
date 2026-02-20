package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultBaseURL = "https://modelslab.com"
	DefaultOutput  = "human"
	DefaultProfile = "default"
)

var (
	configDir  string
	configFile string
)

func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %w", err)
	}

	configDir = filepath.Join(home, ".config", "modelslab")
	configFile = filepath.Join(configDir, "config.toml")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(configDir, "profiles"), 0700); err != nil {
		return fmt.Errorf("could not create profiles directory: %w", err)
	}

	viper.SetConfigFile(configFile)
	viper.SetConfigType("toml")

	// Defaults
	viper.SetDefault("defaults.output", DefaultOutput)
	viper.SetDefault("defaults.base_url", DefaultBaseURL)
	viper.SetDefault("defaults.profile", DefaultProfile)
	viper.SetDefault("generation.default_model", "sdxl")
	viper.SetDefault("generation.auto_download", true)
	viper.SetDefault("generation.output_dir", "./generated")

	// Environment variables
	viper.SetEnvPrefix("MODELSLAB")
	viper.BindEnv("api_key", "MODELSLAB_API_KEY")
	viper.BindEnv("token", "MODELSLAB_TOKEN")
	viper.BindEnv("base_url", "MODELSLAB_BASE_URL")
	viper.BindEnv("profile", "MODELSLAB_PROFILE")
	viper.BindEnv("output", "MODELSLAB_OUTPUT")

	// Read config file (ignore if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only return error if it's not a "file not found" error
			if !os.IsNotExist(err) {
				return nil // Config file doesn't exist yet, that's fine
			}
		}
	}

	// Also check project-level config
	projectConfig := filepath.Join(".modelslab", "config.toml")
	if _, err := os.Stat(projectConfig); err == nil {
		projectViper := viper.New()
		projectViper.SetConfigFile(projectConfig)
		projectViper.SetConfigType("toml")
		if err := projectViper.ReadInConfig(); err == nil {
			viper.MergeConfigMap(projectViper.AllSettings())
		}
	}

	return nil
}

func ConfigDir() string {
	return configDir
}

func ConfigFile() string {
	return configFile
}

func GetBaseURL() string {
	if url := viper.GetString("base_url"); url != "" {
		return url
	}
	if url := viper.GetString("defaults.base_url"); url != "" {
		return url
	}
	return DefaultBaseURL
}

func GetProfile() string {
	if p := viper.GetString("profile"); p != "" {
		return p
	}
	if p := viper.GetString("defaults.profile"); p != "" {
		return p
	}
	return DefaultProfile
}

func GetOutput() string {
	if o := viper.GetString("output"); o != "" {
		return o
	}
	if o := viper.GetString("defaults.output"); o != "" {
		return o
	}
	return DefaultOutput
}

func GetAPIKey() string {
	return viper.GetString("api_key")
}

func Set(key, value string) error {
	viper.Set(key, value)
	return viper.WriteConfigAs(configFile)
}

func Get(key string) string {
	return viper.GetString(key)
}

func AllSettings() map[string]interface{} {
	return viper.AllSettings()
}
