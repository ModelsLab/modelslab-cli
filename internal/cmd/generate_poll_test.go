package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A failed generation comes back as HTTP 200 with status "error" and no job id.
// pollAndDownload used to poll fetch/ with that empty id until the timeout.
func generationCommand(t *testing.T, noWait bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	addGenerationFlags(cmd)
	require.NoError(t, cmd.Flags().Set("timeout", (2*time.Second).String()))
	if noWait {
		require.NoError(t, cmd.Flags().Set("no-wait", "true"))
	}

	return cmd
}

func TestPollAndDownload_ReturnsTheApiErrorWithoutPolling(t *testing.T) {
	for _, noWait := range []bool{false, true} {
		start := time.Now()

		err := pollAndDownload(generationCommand(t, noWait), "video", "/v7/video-fusion/fetch", map[string]interface{}{
			"status":  "error",
			"code":    "provider_error",
			"message": "The provider refused the request.",
		})

		require.Error(t, err)
		assert.Equal(t, "The provider refused the request.", err.Error())
		assert.Less(t, time.Since(start), time.Second, "it must not poll")
	}
}

func TestPollAndDownload_ReportsAStructuredValidationMessage(t *testing.T) {
	err := pollAndDownload(generationCommand(t, false), "image", "/v7/images/fetch", map[string]interface{}{
		"status":  "error",
		"message": map[string]interface{}{"init_image": []interface{}{"The init image field is required."}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "The init image field is required.")
}

func TestPollAndDownload_RefusesToPollWithoutAJobID(t *testing.T) {
	start := time.Now()

	err := pollAndDownload(generationCommand(t, false), "video", "/v7/video-fusion/fetch", map[string]interface{}{
		"status": "processing",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no job id")
	assert.Less(t, time.Since(start), time.Second, "it must not poll")
}
