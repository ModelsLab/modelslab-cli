package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVersionUpdatesRootCommandVersion(t *testing.T) {
	oldVersion := cliVersion
	oldCommit := cliCommit
	oldDate := cliDate
	oldRootVersion := rootCmd.Version
	t.Cleanup(func() {
		SetVersion(oldVersion, oldCommit, oldDate)
		rootCmd.Version = oldRootVersion
	})

	SetVersion("1.2.3", "abc123", "2026-05-24")

	assert.Equal(t, "1.2.3", cliVersion)
	assert.Equal(t, "1.2.3", rootCmd.Version)
	assert.Equal(t, "abc123", cliCommit)
	assert.Equal(t, "2026-05-24", cliDate)
}
