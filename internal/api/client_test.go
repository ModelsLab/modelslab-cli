package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.com", "token123", "apikey123")
	assert.Equal(t, "https://example.com", c.BaseURL)
	assert.Equal(t, "token123", c.Token)
	assert.Equal(t, "apikey123", c.APIKey)
	assert.NotNil(t, c.HTTPClient)
}

func TestSetVersion(t *testing.T) {
	c := NewClient("https://example.com", "", "")
	c.SetVersion("1.2.3")
	assert.Equal(t, "modelslab-cli/1.2.3", c.UserAgent)
}

func TestDoControlPlane_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/agents/v1/me", r.URL.Path)
		assert.Equal(t, "Bearer testtoken", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"name": "Test User", "email": "test@example.com"},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "testtoken", "")
	var result map[string]interface{}
	err := c.DoControlPlane("GET", "/me", nil, &result)

	require.NoError(t, err)
	data := result["data"].(map[string]interface{})
	assert.Equal(t, "Test User", data["name"])
	assert.Equal(t, "test@example.com", data["email"])
}

func TestDoControlPlane_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"message": "Unauthenticated."})
	}))
	defer server.Close()

	c := NewClient(server.URL, "badtoken", "")
	var result map[string]interface{}
	err := c.DoControlPlane("GET", "/me", nil, &result)

	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 401, apiErr.StatusCode)
	assert.Equal(t, ExitAuthError, apiErr.ExitCode)
}

func TestDoControlPlane_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "Not found"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "token", "")
	var result map[string]interface{}
	err := c.DoControlPlane("GET", "/nonexistent", nil, &result)

	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 404, apiErr.StatusCode)
	assert.Equal(t, ExitNotFound, apiErr.ExitCode)
}

func TestDoControlPlane_RateLimited(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(429)
			json.NewEncoder(w).Encode(map[string]string{"message": "Too many requests"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "token", "")
	var result map[string]interface{}
	err := c.DoControlPlane("GET", "/test", nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, 3, attempts) // Should have retried
}

func TestDoControlPlane_PostWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test@example.com", body["email"])
		assert.Equal(t, "password123", body["password"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"token": "newtoken123"},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "")
	var result map[string]interface{}
	err := c.DoControlPlane("POST", "/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}, &result)

	require.NoError(t, err)
	data := result["data"].(map[string]interface{})
	assert.Equal(t, "newtoken123", data["token"])
}

func TestDoGeneration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v7/images/text-to-image", r.URL.Path)

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "apikey123", body["key"])
		assert.Equal(t, "a sunset", body["prompt"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "processing",
			"id":     12345,
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "apikey123")
	var result map[string]interface{}
	err := c.DoGeneration("POST", "/v7/images/text-to-image", map[string]interface{}{
		"key":    "apikey123",
		"prompt": "a sunset",
	}, &result)

	require.NoError(t, err)
	assert.Equal(t, "processing", result["status"])
}

func TestDoControlPlaneIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("Idempotency-Key"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "token", "")
	var result map[string]interface{}
	err := c.DoControlPlaneIdempotent("POST", "/wallet/fund", map[string]interface{}{
		"amount": 25,
	}, &result, "")

	require.NoError(t, err)
}

func TestDoControlPlaneIdempotent_CustomKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "my-custom-key", r.Header.Get("Idempotency-Key"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "token", "")
	var result map[string]interface{}
	err := c.DoControlPlaneIdempotent("POST", "/wallet/fund", nil, &result, "my-custom-key")

	require.NoError(t, err)
}

func TestGenerateIdempotencyKey(t *testing.T) {
	key1 := GenerateIdempotencyKey()
	key2 := GenerateIdempotencyKey()
	assert.NotEmpty(t, key1)
	assert.NotEmpty(t, key2)
	assert.NotEqual(t, key1, key2)
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 401, Message: "Unauthorized", ExitCode: ExitAuthError}
	assert.Equal(t, "API error (HTTP 401): Unauthorized", err.Error())
}

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   int
		wantMsg    string
	}{
		{
			"json with message",
			401,
			`{"message":"Token expired"}`,
			ExitAuthError,
			"Token expired",
		},
		{
			"json with error field",
			400,
			`{"error":"Bad request"}`,
			ExitGeneralError,
			"Bad request",
		},
		{
			"plain text",
			500,
			"Internal Server Error",
			ExitGeneralError,
			"Internal Server Error",
		},
		{
			"404",
			404,
			`{"message":"Not found"}`,
			ExitNotFound,
			"Not found",
		},
		{
			"402 payment",
			402,
			`{"message":"Insufficient balance"}`,
			ExitPaymentError,
			"Insufficient balance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAPIError(tt.statusCode, []byte(tt.body))
			assert.Equal(t, tt.wantCode, err.ExitCode)
			assert.Equal(t, tt.wantMsg, err.Message)
		})
	}
}

func TestDoControlPlane_ServerError_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"message": "Server error"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "token", "")
	var result map[string]interface{}
	err := c.DoControlPlane("GET", "/test", nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
	assert.GreaterOrEqual(t, attempts, 3)
}

// The control plane wraps every failure as {"data":null,"error":{...},"meta":{...}}.
// Reading only the top level used to print the whole JSON blob at the user.
func TestParseAPIError_ControlPlaneEnvelope(t *testing.T) {
	body := `{"data":null,"error":{"code":"invalid_credentials","message":"Email or password is incorrect.","details":[]},"meta":{"request_id":"abc"}}`

	err := parseAPIError(401, []byte(body))

	assert.Equal(t, "Email or password is incorrect.", err.Message)
	assert.Equal(t, "invalid_credentials", err.Code)
	assert.Equal(t, ExitAuthError, err.ExitCode)
}

func TestParseAPIError_ControlPlaneEnvelope_Unverified(t *testing.T) {
	body := `{"data":null,"error":{"code":"email_not_verified","message":"Email is not verified.","details":{"verification_required":true}},"meta":{}}`

	err := parseAPIError(403, []byte(body))

	assert.Equal(t, "Email is not verified.", err.Message)
	assert.Equal(t, "email_not_verified", err.Code)
	assert.Equal(t, ExitAuthError, err.ExitCode)
}

func TestParseAPIError_TruncatesUnrecognisedBody(t *testing.T) {
	body := strings.Repeat("<html>proxy error</html>", 100)

	err := parseAPIError(502, []byte(body))

	assert.LessOrEqual(t, len(err.Message), maxRawErrorBody+len("…"))
	assert.Empty(t, err.Code)
}

func TestParseAPIError_EmptyBody(t *testing.T) {
	err := parseAPIError(500, nil)

	assert.Equal(t, "Request failed with HTTP 500.", err.Message)
}

func TestParseAPIError_KeepsValidationDetails(t *testing.T) {
	body := `{"data":null,"error":{"code":"validation_error","message":"Invalid login payload.","details":{"token_expiry":["The selected token expiry is invalid."],"email":["The email field is required."]}},"meta":{}}`

	err := parseAPIError(422, []byte(body))

	assert.Equal(t, "validation_error", err.Code)
	assert.Equal(t, []string{
		"email: The email field is required.",
		"token_expiry: The selected token expiry is invalid.",
	}, err.FieldErrors())
}

// An empty details bag serialises as [] rather than {}.
func TestParseAPIError_EmptyDetailsBag(t *testing.T) {
	err := parseAPIError(401, []byte(`{"data":null,"error":{"code":"invalid_credentials","message":"Nope.","details":[]}}`))

	assert.Nil(t, err.Details)
	assert.Nil(t, err.FieldErrors())
}

func TestParseAPIError_TruncationLandsOnARuneBoundary(t *testing.T) {
	err := parseAPIError(502, []byte(strings.Repeat("é", 400)))

	assert.True(t, utf8.ValidString(err.Message), "truncated body must stay valid UTF-8")
}

// Laravel's throttle sends Retry-After; the old code read only
// X-RateLimit-Reset and replaced the server's message with a blanket one.
func TestRateLimitWait_PrefersRetryAfter(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "45")

	assert.Equal(t, 45*time.Second, rateLimitWait(header))
}

func TestRateLimitWait_FallsBackToResetTimestamp(t *testing.T) {
	header := http.Header{}
	header.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+12, 10))

	wait := rateLimitWait(header)

	assert.Greater(t, wait, 10*time.Second)
	assert.LessOrEqual(t, wait, 12*time.Second)
}

func TestRateLimitWait_NoHeaders(t *testing.T) {
	assert.Zero(t, rateLimitWait(http.Header{}))
}

// A long Retry-After must surface immediately instead of blocking the terminal,
// and must keep the server's own wording and window.
func TestDoControlPlane_LongRateLimitReturnsImmediately(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		w.Write([]byte(`{"data":null,"error":{"code":"rate_limited","message":"Too Many Attempts."}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", "")
	err := client.DoControlPlane("GET", "/api-keys", nil, nil)

	require.Error(t, err)
	apiErr := err.(*APIError)
	assert.Equal(t, ExitRateLimited, apiErr.ExitCode)
	assert.Contains(t, apiErr.Message, "Too Many Attempts.")
	assert.Contains(t, apiErr.Message, "1m0s")
	assert.Equal(t, 1, attempts, "a 60s window must not be slept through")
}

// A proxy error page is HTML; its <title> is the only part worth a terminal line.
func TestParseAPIError_SummarisesAProxyErrorPage(t *testing.T) {
	body := `<html><head><title>502 Bad Gateway</title></head><body><center><h1>502 Bad Gateway</h1></center><hr><center>nginx</center></body></html>`

	err := parseAPIError(502, []byte(body))

	assert.Equal(t, "502 Bad Gateway (HTTP 502)", err.Message)
}
