package cmd

import (
	"testing"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func hintsFor(code string, status int, details map[string][]string) []string {
	return loginFailureHints(&api.APIError{Code: code, StatusCode: status, Details: details}, "maxx@example.com")
}

// An OAuth-only account has no password, so "check your email and password" is a
// dead end — the hint has to name the path that can actually work.
func TestLoginFailureHints_InvalidCredentialsPointsAtBrowserLogin(t *testing.T) {
	hints := hintsFor("invalid_credentials", 401, nil)

	assert.Contains(t, joined(hints), "auth login --browser")
	assert.Contains(t, joined(hints), "forgot-password")
}

// A proxy 502 carries no error code. Telling that user their password is wrong
// is the multi-day rabbit hole this whole ticket is about.
func TestLoginFailureHints_UnrecognisedErrorIsNotBlamedOnCredentials(t *testing.T) {
	hints := hintsFor("", 502, nil)

	assert.NotContains(t, joined(hints), "Check your email and password")
	assert.NotContains(t, joined(hints), "forgot-password")
	assert.Contains(t, joined(hints), "502")
	assert.Contains(t, joined(hints), "network or service problem")
}

func TestLoginFailureHints_UnverifiedEmailOffersResend(t *testing.T) {
	hints := hintsFor("email_not_verified", 403, nil)

	assert.Contains(t, joined(hints), "resend-verification --email maxx@example.com")
}

// The old hint blamed --email whatever the server actually rejected.
func TestLoginFailureHints_ValidationErrorNamesTheRejectedField(t *testing.T) {
	hints := hintsFor("validation_error", 422, map[string][]string{
		"token_expiry": {"The selected token expiry is invalid."},
	})

	assert.Contains(t, joined(hints), "token_expiry: The selected token expiry is invalid.")
	assert.NotContains(t, joined(hints), "Check the email address")
}

func TestLoginFailureHints_ValidationErrorWithoutDetailsStillGuides(t *testing.T) {
	hints := hintsFor("validation_error", 422, nil)

	assert.Contains(t, joined(hints), "--email")
}

func joined(hints []string) string {
	out := ""
	for _, hint := range hints {
		out += hint + "\n"
	}
	return out
}
