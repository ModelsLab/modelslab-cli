package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "modelslab-cli"
)

type Credentials struct {
	Token  string `json:"token,omitempty"`
	APIKey string `json:"api_key,omitempty"`
	Email  string `json:"email,omitempty"`
}

// StoreToken stores a bearer token for the given profile in the system keychain.
// Falls back to file-based storage if keychain is unavailable.
func StoreToken(profile, token string) error {
	err := keyring.Set(serviceName, profile+":token", token)
	if err != nil {
		return storeToFile(profile, "token", token)
	}
	return nil
}

// GetToken retrieves the bearer token for the given profile.
func GetToken(profile string) (string, error) {
	token, err := keyring.Get(serviceName, profile+":token")
	if err != nil {
		return getFromFile(profile, "token")
	}
	return token, nil
}

// StoreAPIKey stores an API key for the given profile.
func StoreAPIKey(profile, apiKey string) error {
	err := keyring.Set(serviceName, profile+":apikey", apiKey)
	if err != nil {
		return storeToFile(profile, "apikey", apiKey)
	}
	return nil
}

// GetAPIKey retrieves the API key for the given profile.
func GetAPIKey(profile string) (string, error) {
	key, err := keyring.Get(serviceName, profile+":apikey")
	if err != nil {
		return getFromFile(profile, "apikey")
	}
	return key, nil
}

// StoreEmail stores the email for the given profile.
func StoreEmail(profile, email string) error {
	err := keyring.Set(serviceName, profile+":email", email)
	if err != nil {
		return storeToFile(profile, "email", email)
	}
	return nil
}

// GetEmail retrieves the email for the given profile.
func GetEmail(profile string) (string, error) {
	email, err := keyring.Get(serviceName, profile+":email")
	if err != nil {
		return getFromFile(profile, "email")
	}
	return email, nil
}

// DeleteProfile removes all credentials for a profile.
func DeleteProfile(profile string) error {
	keyring.Delete(serviceName, profile+":token")
	keyring.Delete(serviceName, profile+":apikey")
	keyring.Delete(serviceName, profile+":email")
	// Also remove file-based fallback
	deleteFileCredentials(profile)
	return nil
}

// File-based fallback for environments without keychain

func credentialFilePath(profile string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "modelslab", "profiles", profile+".json")
}

func storeToFile(profile, key, value string) error {
	path := credentialFilePath(profile)
	creds := make(map[string]string)

	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &creds)
	}

	creds[key] = value
	data, err = json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal credentials: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func getFromFile(profile, key string) (string, error) {
	path := credentialFilePath(profile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("no credentials found for profile %q", profile)
	}

	creds := make(map[string]string)
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("could not parse credentials: %w", err)
	}

	val, ok := creds[key]
	if !ok {
		return "", fmt.Errorf("no %s found for profile %q", key, profile)
	}
	return val, nil
}

func deleteFileCredentials(profile string) {
	path := credentialFilePath(profile)
	os.Remove(path)
}
