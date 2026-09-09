package cmd

import (
	"errors"
	"os"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/ModelsLab/modelslab-cli/internal/auth"
	"github.com/ModelsLab/modelslab-cli/internal/output"
)

// ReportError prints a failed command's error and returns the process exit code.
//
// Only `auth login` used to map an APIError to its documented exit code; every
// other command exited 1, so a script could not tell an expired token (3) from a
// bad flag (2) or a rate limit (4). And a 401 was reported as a bare
// "Unauthenticated." with no hint that the CLI had simply never been logged in —
// which is what it looks like when the keychain is locked and getClient()
// silently sends no Authorization header at all.
func ReportError(err error) int {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		output.PrintError(err.Error())
		return api.ExitGeneralError
	}

	output.PrintError(apiErr.Message, commandFailureHints(apiErr)...)

	return apiErr.ExitCode
}

func commandFailureHints(apiErr *api.APIError) []string {
	hints := apiErr.FieldErrors()

	if apiErr.ExitCode == api.ExitAuthError && !hasStoredCredentials() {
		hints = append(hints,
			"No credentials are stored for profile "+flagProfile+".",
			"Run: modelslab auth login --browser",
			"If you are logged in, your OS keychain may be locked — check: modelslab auth status",
		)
	}

	return hints
}

func hasStoredCredentials() bool {
	if os.Getenv("MODELSLAB_TOKEN") != "" || flagAPIKey != "" {
		return true
	}
	if token, err := auth.GetToken(flagProfile); err == nil && token != "" {
		return true
	}
	key, err := auth.GetAPIKey(flagProfile)

	return err == nil && key != ""
}
