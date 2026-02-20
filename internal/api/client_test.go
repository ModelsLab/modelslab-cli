package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
