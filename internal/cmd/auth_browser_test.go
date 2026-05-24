package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBrowserLoginURL(t *testing.T) {
	loginURL, err := buildBrowserLoginURL(
		"https://modelslab.com/",
		"http://127.0.0.1:4545/callback",
		"state-123",
		"modelslab-cli@test-host",
		"6_months",
	)

	require.NoError(t, err)

	parsed, err := url.Parse(loginURL)
	require.NoError(t, err)

	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "modelslab.com", parsed.Host)
	assert.Equal(t, "/auth/modelslab-cli/oauth/authorize", parsed.Path)
	assert.Equal(t, "ModelsLab CLI", parsed.Query().Get("client_name"))
	assert.Equal(t, "modelslab-cli@test-host", parsed.Query().Get("device_name"))
	assert.Equal(t, "http://127.0.0.1:4545/callback", parsed.Query().Get("redirect_uri"))
	assert.Equal(t, "state-123", parsed.Query().Get("state"))
	assert.Equal(t, "6_months", parsed.Query().Get("token_expiry"))
}

func TestBrowserLoginCallbackHandlerAcceptsValidPost(t *testing.T) {
	callbacks := make(chan browserLoginCallback, 1)
	form := url.Values{
		"access_token":           {"token-123"},
		"api_key":                {"key-123"},
		"email":                  {"user@example.com"},
		"expires_at":             {"2026-06-24T00:00:00Z"},
		"model_id":               {"modelslab-cli"},
		"state":                  {"state-123"},
		"token_expiry":           {"1_month"},
		"token_expiry_effective": {"1_month"},
		"token_lifetime_capped":  {"0"},
		"token_type":             {"Bearer"},
	}
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	browserLoginCallbackHandler("state-123", callbacks)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	select {
	case callback := <-callbacks:
		assert.Equal(t, "token-123", callback.AccessToken)
		assert.Equal(t, "key-123", callback.APIKey)
		assert.Equal(t, "user@example.com", callback.Email)
		assert.Equal(t, "state-123", callback.State)
	default:
		t.Fatal("expected callback result")
	}
}

func TestBrowserLoginCallbackHandlerRejectsInvalidState(t *testing.T) {
	callbacks := make(chan browserLoginCallback, 1)
	form := url.Values{
		"api_key": {"key-123"},
		"state":   {"wrong-state"},
	}
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	browserLoginCallbackHandler("state-123", callbacks)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, callbacks)
}
