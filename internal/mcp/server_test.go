package mcp

import (
	"testing"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/stretchr/testify/assert"
)

// The client is built once in `mcp serve` and lives for the whole process.
// auth-login used to return the token without ever assigning it, so an agent
// that logged in through MCP got 401 from every other tool for the life of the
// process — with a valid token sitting in its own transcript.
//
// profile is empty here so the test only touches the in-memory client and never
// writes to the developer's real keychain.
func TestApplyCredentials_UpdatesTheLiveClient(t *testing.T) {
	s := &Server{client: api.NewClient("https://modelslab.com", "", "")}

	s.applyCredentials(map[string]interface{}{
		"data": map[string]interface{}{
			"access_token": "tok-123",
			"api_key":      "ml-abc",
		},
	})

	assert.Equal(t, "tok-123", s.client.Token)
	assert.Equal(t, "ml-abc", s.client.APIKey)
}

func TestApplyCredentials_AcceptsTheLegacyTokenField(t *testing.T) {
	s := &Server{client: api.NewClient("https://modelslab.com", "", "")}

	s.applyCredentials(map[string]interface{}{
		"data": map[string]interface{}{"token": "tok-legacy"},
	})

	assert.Equal(t, "tok-legacy", s.client.Token)
}

// api-keys-create returns the new key under "key".
func TestApplyCredentials_PicksUpANewlyCreatedApiKey(t *testing.T) {
	s := &Server{client: api.NewClient("https://modelslab.com", "tok", "ml-old")}

	s.applyCredentials(map[string]interface{}{
		"data": map[string]interface{}{"key": "ml-new"},
	})

	assert.Equal(t, "ml-new", s.client.APIKey)
	assert.Equal(t, "tok", s.client.Token)
}

func TestApplyCredentials_LeavesTheClientAloneOnAnUnexpectedShape(t *testing.T) {
	s := &Server{client: api.NewClient("https://modelslab.com", "tok", "ml-old")}

	s.applyCredentials(map[string]interface{}{"data": "not-a-map"})
	s.applyCredentials(map[string]interface{}{})

	assert.Equal(t, "tok", s.client.Token)
	assert.Equal(t, "ml-old", s.client.APIKey)
}
