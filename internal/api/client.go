package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Exit codes matching the design document
const (
	ExitSuccess      = 0
	ExitGeneralError = 1
	ExitUsageError   = 2
	ExitAuthError    = 3
	ExitRateLimited  = 4
	ExitNotFound     = 5
	ExitPaymentError = 6
	ExitGenTimeout   = 7
	ExitNetworkError = 10
)

// maxRawErrorBody caps how much of an unrecognised error body is echoed back.
const maxRawErrorBody = 300

// htmlTitle pulls the <title> out of a proxy's error page.
var htmlTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// maxRateLimitWait is the longest window the client will sleep through before it
// gives the terminal back. Laravel's throttle routinely names 60s; blocking that
// long without a word looks like a hang.
const maxRateLimitWait = 30 * time.Second

// rateLimitWait reads how long the server asked us to wait. Retry-After is the
// standard header and the one Laravel's throttle middleware sends;
// X-RateLimit-Reset is an absolute unix timestamp.
func rateLimitWait(header http.Header) time.Duration {
	if value := header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	if value := header.Get("X-RateLimit-Reset"); value != "" {
		if resetAt, err := strconv.ParseInt(value, 10, 64); err == nil {
			if seconds := resetAt - time.Now().Unix(); seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}

	return 0
}

type Client struct {
	BaseURL    string
	Token      string // Bearer token for control plane
	APIKey     string // API key for generation
	HTTPClient *http.Client
	UserAgent  string
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	// Details carries the control plane's per-field validation errors, keyed by
	// field name. Dropping it left the CLI guessing which field the server
	// actually rejected.
	Details  map[string][]string
	ExitCode int
}

// FieldErrors renders Details as "field: message" lines, sorted for stable output.
func (e *APIError) FieldErrors() []string {
	if len(e.Details) == 0 {
		return nil
	}

	fields := make([]string, 0, len(e.Details))
	for field := range e.Details {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		for _, message := range e.Details[field] {
			lines = append(lines, field+": "+message)
		}
	}
	return lines
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
			retryAfter := rateLimitWait(resp.Header)
			if attempt < maxRetries && retryAfter <= maxRateLimitWait {
				waitTime := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				if retryAfter > 0 {
					waitTime = retryAfter
				}
				time.Sleep(waitTime)
				continue
			}
			// Keep the server's own message and the window it named. The old
			// blanket "try again later" sent people straight back into the wall.
			apiErr := parseAPIError(429, respBody)
			if retryAfter > 0 {
				apiErr.Message = fmt.Sprintf("%s Retry in %s.", apiErr.Message, retryAfter)
			}
			return apiErr
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

	code, message, details := extractAPIErrorFields(body)
	if message == "" {
		message = summarizeRawBody(body, statusCode)
	}

	return &APIError{StatusCode: statusCode, Code: code, Message: message, Details: details, ExitCode: exitCode}
}

// extractAPIErrorFields pulls the machine code and the human message out of an
// error body. The control plane wraps every failure as
//
//	{"data":null,"error":{"code":"invalid_credentials","message":"..."},"meta":{...}}
//
// so the message is NOT at the top level and "error" is an object, not a string.
// Reading only the top level printed the whole JSON blob back at the user on
// every failed login. The flatter shapes are still handled: some legacy
// endpoints answer {"message":...} or {"error":"..."}.
func extractAPIErrorFields(body []byte) (string, string, map[string][]string) {
	var errResp map[string]interface{}
	if err := json.Unmarshal(body, &errResp); err != nil {
		return "", "", nil
	}

	if errObj, ok := errResp["error"].(map[string]interface{}); ok {
		code, _ := errObj["code"].(string)
		message, _ := errObj["message"].(string)
		if message == "" {
			message, _ = errResp["message"].(string)
		}
		return code, message, parseErrorDetails(errObj["details"])
	}

	if msg, ok := errResp["message"].(string); ok && msg != "" {
		code, _ := errResp["code"].(string)
		return code, msg, parseErrorDetails(errResp["errors"])
	}

	if msg, ok := errResp["error"].(string); ok && msg != "" {
		code, _ := errResp["code"].(string)
		return code, msg, parseErrorDetails(errResp["errors"])
	}

	return "", "", nil
}

// parseErrorDetails normalises Laravel's per-field error bag. It arrives as
// {"field": ["message", ...]}, but an empty bag serialises as [] rather than {},
// and some codes put a flat object there instead.
func parseErrorDetails(raw interface{}) map[string][]string {
	bag, ok := raw.(map[string]interface{})
	if !ok || len(bag) == 0 {
		return nil
	}

	details := make(map[string][]string, len(bag))
	for field, value := range bag {
		switch typed := value.(type) {
		case string:
			details[field] = []string{typed}
		case []interface{}:
			for _, item := range typed {
				if message, ok := item.(string); ok {
					details[field] = append(details[field], message)
				}
			}
		}
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// summarizeRawBody is the last resort when the body carries no message we
// recognise. An unparseable body is often an HTML error page from a proxy, and
// dumping the whole thing into the terminal helps nobody.
func summarizeRawBody(body []byte, statusCode int) string {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return fmt.Sprintf("Request failed with HTTP %d.", statusCode)
	}

	// A proxy error page is HTML. Its <title> ("502 Bad Gateway") is the only
	// part worth showing; the markup around it is noise in a terminal.
	if match := htmlTitle.FindSubmatch(body); match != nil {
		if title := strings.TrimSpace(string(match[1])); title != "" {
			return fmt.Sprintf("%s (HTTP %d)", title, statusCode)
		}
	}

	if len(raw) > maxRawErrorBody {
		// Slice on a rune boundary; a body cut mid-rune renders as U+FFFD.
		cut := maxRawErrorBody
		for cut > 0 && !utf8.RuneStart(raw[cut]) {
			cut--
		}
		return raw[:cut] + "…"
	}
	return raw
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
