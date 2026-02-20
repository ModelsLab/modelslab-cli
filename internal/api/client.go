package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Exit codes matching the design document
const (
	ExitSuccess       = 0
	ExitGeneralError  = 1
	ExitUsageError    = 2
	ExitAuthError     = 3
	ExitRateLimited   = 4
	ExitNotFound      = 5
	ExitPaymentError  = 6
	ExitGenTimeout    = 7
	ExitNetworkError  = 10
)

type Client struct {
	BaseURL    string
	Token      string // Bearer token for control plane
	APIKey     string // API key for generation
	HTTPClient *http.Client
	UserAgent  string
}

type APIError struct {
	StatusCode int
	Message    string
	ExitCode   int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Message)
}

func NewClient(baseURL, token, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		UserAgent: "modelslab-cli/dev",
	}
}

func (c *Client) SetVersion(version string) {
	c.UserAgent = "modelslab-cli/" + version
}

// DoControlPlane makes an authenticated request to the control plane API (Bearer token).
func (c *Client) DoControlPlane(method, path string, body interface{}, result interface{}) error {
	return c.doRequest(method, "/api/agents/v1"+path, body, result, "bearer")
}

// DoGeneration makes an authenticated request to the generation API (API key).
func (c *Client) DoGeneration(method, path string, body interface{}, result interface{}) error {
	return c.doRequest(method, "/api"+path, body, result, "apikey")
}

// DoStripe makes a request to the Stripe API for card tokenization.
func (c *Client) DoStripe(publishableKey string, body map[string]string) (map[string]interface{}, error) {
	formData := ""
	for k, v := range body {
		if formData != "" {
			formData += "&"
		}
		formData += k + "=" + v
	}

	req, err := http.NewRequest("POST", "https://api.stripe.com/v1/payment_methods", strings.NewReader(formData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+publishableKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &APIError{StatusCode: 0, Message: err.Error(), ExitCode: ExitNetworkError}
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		msg := "Stripe API error"
		if errObj, ok := result["error"].(map[string]interface{}); ok {
			if m, ok := errObj["message"].(string); ok {
				msg = m
			}
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg, ExitCode: ExitPaymentError}
	}

	return result, nil
}

func (c *Client) doRequest(method, path string, body interface{}, result interface{}, authType string) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("could not marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.BaseURL + path
	maxRetries := 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("could not create request: %w", err)
		}

		// Reset body reader for retries
		if body != nil && attempt > 0 {
			jsonBody, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(jsonBody)
			req.Body = io.NopCloser(bodyReader)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.UserAgent)

		switch authType {
		case "bearer":
			if c.Token != "" {
				req.Header.Set("Authorization", "Bearer "+c.Token)
			}
		case "apikey":
			if c.APIKey != "" {
				req.Header.Set("key", c.APIKey)
			}
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return &APIError{StatusCode: 0, Message: err.Error(), ExitCode: ExitNetworkError}
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("could not read response body: %w", err)
		}

		// Handle rate limiting
		if resp.StatusCode == 429 {
			if attempt < maxRetries {
				waitTime := math.Pow(2, float64(attempt))
				if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
					if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
						waitSecs := resetTime - time.Now().Unix()
						if waitSecs > 0 && waitSecs < 30 {
							waitTime = float64(waitSecs)
						}
					}
				}
				time.Sleep(time.Duration(waitTime) * time.Second)
				continue
			}
			return &APIError{StatusCode: 429, Message: "Rate limited. Please try again later.", ExitCode: ExitRateLimited}
		}

		// Handle server errors with retry
		if resp.StatusCode >= 500 && attempt < maxRetries {
			waitTime := math.Pow(2, float64(attempt))
			time.Sleep(time.Duration(waitTime) * time.Second)
			continue
		}

		// Handle error responses
		if resp.StatusCode >= 400 {
			return parseAPIError(resp.StatusCode, respBody)
		}

		// Parse successful response
		if result != nil {
			if err := json.Unmarshal(respBody, result); err != nil {
				// If we can't unmarshal into the target, try returning raw
				if rawResult, ok := result.(*json.RawMessage); ok {
					*rawResult = respBody
				} else {
					return fmt.Errorf("could not parse response: %w", err)
				}
			}
		}

		return nil
	}

	return &APIError{StatusCode: 0, Message: "max retries exceeded", ExitCode: ExitNetworkError}
}

func parseAPIError(statusCode int, body []byte) *APIError {
	exitCode := ExitGeneralError
	switch statusCode {
	case 401, 403:
		exitCode = ExitAuthError
	case 404:
		exitCode = ExitNotFound
	case 402:
		exitCode = ExitPaymentError
	case 429:
		exitCode = ExitRateLimited
	}

	// Try to extract message from JSON response
	var errResp map[string]interface{}
	message := string(body)
	if err := json.Unmarshal(body, &errResp); err == nil {
		if msg, ok := errResp["message"].(string); ok {
			message = msg
		} else if msg, ok := errResp["error"].(string); ok {
			message = msg
		}
	}

	return &APIError{StatusCode: statusCode, Message: message, ExitCode: exitCode}
}

// GenerateIdempotencyKey creates a UUID for idempotent billing operations.
func GenerateIdempotencyKey() string {
	return uuid.New().String()
}

// DoControlPlaneIdempotent makes an idempotent control plane request.
func (c *Client) DoControlPlaneIdempotent(method, path string, body interface{}, result interface{}, idempotencyKey string) error {
	if idempotencyKey == "" {
		idempotencyKey = GenerateIdempotencyKey()
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("could not marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.BaseURL + "/api/agents/v1" + path

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Idempotency-Key", idempotencyKey)

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return &APIError{StatusCode: 0, Message: err.Error(), ExitCode: ExitNetworkError}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("could not parse response: %w", err)
		}
	}

	return nil
}

// IsTerminal checks if stdout is a terminal.
func IsTerminal() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}
