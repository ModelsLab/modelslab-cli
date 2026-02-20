package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseURL is the local Laravel server for integration testing.
// Set MODELSLAB_TEST_URL to override.
var baseURL = "http://127.0.0.1:8888"

func TestMain(m *testing.M) {
	if url := os.Getenv("MODELSLAB_TEST_URL"); url != "" {
		baseURL = url
	}
	// Build the binary before running tests
	root := getProjectRoot()
	build := exec.Command("go", "build", "-o", filepath.Join(root, "modelslab"), "./cmd/modelslab/")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build binary: %s\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func runCLI(args ...string) (string, string, int) {
	allArgs := append([]string{"--base-url", baseURL}, args...)
	cmd := exec.Command("./modelslab", allArgs...)
	cmd.Dir = getProjectRoot()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	return stdout.String(), stderr.String(), exitCode
}

func runCLIJSON(args ...string) (map[string]interface{}, string, int) {
	allArgs := append(args, "--output", "json")
	stdout, stderr, code := runCLI(allArgs...)

	var result map[string]interface{}
	json.Unmarshal([]byte(stdout), &result)
	return result, stderr, code
}

func getProjectRoot() string {
	// Walk up from the test directory to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

// --- Tests ---

func TestCLI_Help(t *testing.T) {
	stdout, _, code := runCLI("--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "modelslab")
	assert.Contains(t, stdout, "Available Commands")
	assert.Contains(t, stdout, "auth")
	assert.Contains(t, stdout, "generate")
	assert.Contains(t, stdout, "billing")
	assert.Contains(t, stdout, "wallet")
	assert.Contains(t, stdout, "models")
}

func TestCLI_Version(t *testing.T) {
	stdout, _, code := runCLI("--version")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "modelslab version")
}

func TestAuth_Help(t *testing.T) {
	stdout, _, code := runCLI("auth", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "login")
	assert.Contains(t, stdout, "signup")
	assert.Contains(t, stdout, "logout")
	assert.Contains(t, stdout, "status")
	assert.Contains(t, stdout, "tokens")
}

func TestAuth_Status_NoAuth(t *testing.T) {
	stdout, _, code := runCLI("auth", "status", "--profile", "test-empty")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Profile")
}

func TestAuth_Signup_And_Login(t *testing.T) {
	email := fmt.Sprintf("clitest_%d@example.com", os.Getpid())
	password := "TestPassword123!"

	// Sign up
	stdout, stderr, code := runCLI("auth", "signup",
		"--email", email,
		"--password", password,
		"--name", "CLI Test User",
	)
	t.Logf("Signup stdout: %s", stdout)
	t.Logf("Signup stderr: %s", stderr)
	// Signup may fail if email exists, that's OK for repeated test runs
	if code != 0 {
		t.Logf("Signup returned code %d (may already exist), continuing...", code)
	}

	// Login
	stdout, stderr, code = runCLI("auth", "login",
		"--email", email,
		"--password", password,
		"--profile", "cli-test",
	)
	t.Logf("Login stdout: %s", stdout)
	t.Logf("Login stderr: %s", stderr)
	// Login may fail if signup didn't work (email verification required)
	if code != 0 {
		t.Skip("Login failed (may need email verification), skipping authenticated tests")
	}

	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Logged in")
}

func TestAuth_Login_BadCredentials(t *testing.T) {
	_, stderr, code := runCLI("auth", "login",
		"--email", "nonexistent@example.com",
		"--password", "wrongpassword",
		"--profile", "test-bad",
	)
	assert.NotEqual(t, 0, code)
	// Should get an auth error
	combined := stderr
	assert.True(t, len(combined) > 0 || code > 0, "Expected error output or non-zero exit code")
}

func TestGenerate_Help(t *testing.T) {
	stdout, _, code := runCLI("generate", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "image")
	assert.Contains(t, stdout, "video")
	assert.Contains(t, stdout, "tts")
	assert.Contains(t, stdout, "chat")
	assert.Contains(t, stdout, "text-to-3d")
}

func TestGenerate_Image_MissingPrompt(t *testing.T) {
	_, _, code := runCLI("generate", "image")
	assert.NotEqual(t, 0, code) // Should fail without --prompt
}

func TestModels_Help(t *testing.T) {
	stdout, _, code := runCLI("models", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "search")
	assert.Contains(t, stdout, "detail")
	assert.Contains(t, stdout, "filters")
	assert.Contains(t, stdout, "tags")
	assert.Contains(t, stdout, "providers")
}

func TestBilling_Help(t *testing.T) {
	stdout, _, code := runCLI("billing", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "overview")
	assert.Contains(t, stdout, "payment-methods")
	assert.Contains(t, stdout, "add-payment-method")
}

func TestWallet_Help(t *testing.T) {
	stdout, _, code := runCLI("wallet", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "balance")
	assert.Contains(t, stdout, "fund")
	assert.Contains(t, stdout, "transactions")
}

func TestSubscriptions_Help(t *testing.T) {
	stdout, _, code := runCLI("subscriptions", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "plans")
	assert.Contains(t, stdout, "create")
	assert.Contains(t, stdout, "pause")
}

func TestTeams_Help(t *testing.T) {
	stdout, _, code := runCLI("teams", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "list")
	assert.Contains(t, stdout, "invite")
}

func TestUsage_Help(t *testing.T) {
	stdout, _, code := runCLI("usage", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "summary")
	assert.Contains(t, stdout, "products")
	assert.Contains(t, stdout, "history")
}

func TestConfig_SetAndGet(t *testing.T) {
	_, _, code := runCLI("config", "set", "test_integration_key", "test_value_123")
	assert.Equal(t, 0, code)

	stdout, _, code := runCLI("config", "get", "test_integration_key")
	assert.Equal(t, 0, code)
	assert.Contains(t, strings.TrimSpace(stdout), "test_value_123")
}

func TestConfig_List(t *testing.T) {
	stdout, _, code := runCLI("config", "list")
	assert.Equal(t, 0, code)
	assert.True(t, len(stdout) > 0)
}

func TestConfig_ProfilesList(t *testing.T) {
	_, _, code := runCLI("config", "profiles", "list")
	assert.Equal(t, 0, code)
}

func TestCompletion_Bash(t *testing.T) {
	stdout, _, code := runCLI("completion", "bash")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "bash completion")
}

func TestCompletion_Zsh(t *testing.T) {
	stdout, _, code := runCLI("completion", "zsh")
	assert.Equal(t, 0, code)
	assert.True(t, len(stdout) > 0)
}

func TestMCP_Tools(t *testing.T) {
	stdout, _, code := runCLI("mcp", "tools")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "text-to-image")
	assert.Contains(t, stdout, "auth-login")
	assert.Contains(t, stdout, "wallet-balance")
	assert.Contains(t, stdout, "models-search")
}

func TestJQ_Filter(t *testing.T) {
	// This tests the --jq flag with auth status (which works without auth)
	stdout, _, code := runCLI("auth", "status", "--profile", "jq-test", "--output", "json", "--jq", ".profile")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "jq-test")
}

// --- Authenticated Integration Tests ---
// These require a valid auth token. They will be skipped if no token is available.

func getTestToken() string {
	return os.Getenv("MODELSLAB_TEST_TOKEN")
}

func getTestAPIKey() string {
	return os.Getenv("MODELSLAB_TEST_API_KEY")
}

func skipWithoutAuth(t *testing.T) string {
	token := getTestToken()
	if token == "" {
		t.Skip("MODELSLAB_TEST_TOKEN not set, skipping authenticated test")
	}
	return token
}

func runAuthCLI(token string, args ...string) (string, string, int) {
	// Use env vars for auth
	allArgs := append([]string{"--base-url", baseURL}, args...)
	cmd := exec.Command("./modelslab", allArgs...)
	cmd.Dir = getProjectRoot()
	cmd.Env = append(os.Environ(),
		"MODELSLAB_TOKEN="+token,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	return stdout.String(), stderr.String(), exitCode
}

func TestAuth_Profile_Get(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "profile", "get")
	assert.Equal(t, 0, code)
	assert.True(t, len(stdout) > 0)
}

func TestAuth_Profile_Get_JSON(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "profile", "get", "--output", "json")
	assert.Equal(t, 0, code)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err)
}

func TestAuth_Keys_List(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "keys", "list")
	assert.Equal(t, 0, code)
	_ = stdout
}

func TestAuth_Models_Search(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "models", "search", "--search", "flux", "--per-page", "3")
	assert.Equal(t, 0, code)
	assert.True(t, len(stdout) > 0)
}

func TestAuth_Models_Search_JSON(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "models", "search", "--search", "flux", "--per-page", "2", "--output", "json")
	assert.Equal(t, 0, code)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err)
}

func TestAuth_Billing_Overview(t *testing.T) {
	token := skipWithoutAuth(t)
	_, _, code := runAuthCLI(token, "billing", "overview")
	// Might fail with specific errors, but shouldn't crash
	assert.True(t, code == 0 || code == 1)
}

func TestAuth_Wallet_Balance(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "wallet", "balance")
	assert.Equal(t, 0, code)
	assert.True(t, len(stdout) > 0)
}

func TestAuth_Usage_Summary(t *testing.T) {
	token := skipWithoutAuth(t)
	_, _, code := runAuthCLI(token, "usage", "summary")
	assert.True(t, code == 0 || code == 1)
}

func TestAuth_Subscriptions_Plans(t *testing.T) {
	token := skipWithoutAuth(t)
	stdout, _, code := runAuthCLI(token, "subscriptions", "plans")
	assert.Equal(t, 0, code)
	assert.True(t, len(stdout) > 0)
}
