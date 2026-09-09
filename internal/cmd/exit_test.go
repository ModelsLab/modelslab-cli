package cmd

import (
	"errors"
	"testing"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/stretchr/testify/assert"
)

// Only `auth login` used to map an APIError to its documented exit code; every
// other command exited 1, so a script could not tell an expired token from a
// bad flag.
func TestReportError_UsesTheApiExitCode(t *testing.T) {
	cases := map[int]error{
		api.ExitAuthError:    &api.APIError{StatusCode: 401, Message: "Unauthenticated.", ExitCode: api.ExitAuthError},
		api.ExitRateLimited:  &api.APIError{StatusCode: 429, Message: "Too Many Attempts.", ExitCode: api.ExitRateLimited},
		api.ExitNotFound:     &api.APIError{StatusCode: 404, Message: "Not found.", ExitCode: api.ExitNotFound},
		api.ExitPaymentError: &api.APIError{StatusCode: 402, Message: "Insufficient balance.", ExitCode: api.ExitPaymentError},
		api.ExitGeneralError: errors.New("something else"),
	}

	for want, err := range cases {
		assert.Equal(t, want, ReportError(err))
	}
}

func TestReportError_WrappedApiErrorStillMaps(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), &api.APIError{StatusCode: 401, ExitCode: api.ExitAuthError})

	assert.Equal(t, api.ExitAuthError, ReportError(wrapped))
}
