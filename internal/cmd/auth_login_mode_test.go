package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// A Google or GitHub sign-up has no password. A bare `auth login` used to prompt
// for one anyway and die with "password: no input provided".
func TestDefaultToBrowserLogin_BareLoginInATerminalUsesTheBrowser(t *testing.T) {
	assert.True(t, defaultToBrowserLogin("", "", true, "windows", env(nil)))
	assert.True(t, defaultToBrowserLogin("", "", true, "darwin", env(nil)))
	assert.True(t, defaultToBrowserLogin("", "", true, "linux", env(map[string]string{"DISPLAY": ":0"})))
	assert.True(t, defaultToBrowserLogin("", "", true, "linux", env(map[string]string{"WAYLAND_DISPLAY": "wayland-0"})))
}

func TestDefaultToBrowserLogin_CredentialFlagsKeepEmailAndPassword(t *testing.T) {
	assert.False(t, defaultToBrowserLogin("ada@example.com", "", true, "darwin", env(nil)))
	assert.False(t, defaultToBrowserLogin("", "hunter2", true, "darwin", env(nil)))
}

// A script piping credentials must never start waiting on a browser.
func TestDefaultToBrowserLogin_NonInteractiveStdinKeepsEmailAndPassword(t *testing.T) {
	assert.False(t, defaultToBrowserLogin("", "", false, "darwin", env(nil)))
}

// The callback listens on 127.0.0.1, which a browser across an SSH session cannot
// reach — the login would hang until it timed out.
func TestDefaultToBrowserLogin_NoLocalBrowserKeepsEmailAndPassword(t *testing.T) {
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		assert.False(t, defaultToBrowserLogin("", "", true, "darwin", env(map[string]string{key: "x"})), key)
	}

	assert.False(t, defaultToBrowserLogin("", "", true, "linux", env(nil)))
}

func TestErrNoPasswordEntered_PointsAtBrowserLogin(t *testing.T) {
	assert.Contains(t, errNoPasswordEntered.Error(), "modelslab auth login --browser")
	assert.Contains(t, errNoPasswordEntered.Error(), "forgot-password")
}
