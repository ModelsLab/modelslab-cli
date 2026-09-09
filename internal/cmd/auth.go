package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/ModelsLab/modelslab-cli/internal/auth"
	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Login, logout, and manage authentication tokens and profiles.",
}

type browserLoginCallback struct {
	AccessToken          string
	APIKey               string
	Email                string
	Error                string
	ExpiresAt            string
	ModelID              string
	State                string
	TokenExpiry          string
	TokenExpiryEffective string
	TokenLifetimeCapped  string
	TokenType            string
}

// --- auth login ---
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to ModelsLab",
	RunE: func(cmd *cobra.Command, args []string) error {
		useBrowser, _ := cmd.Flags().GetBool("browser")
		if useBrowser {
			return runBrowserLogin(cmd)
		}

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		expiry, _ := cmd.Flags().GetString("expiry")
		deviceName, _ := cmd.Flags().GetString("device-name")

		if email == "" {
			value, err := promptLine("Email: ")
			if err != nil {
				return err
			}
			email = value
		}
		if password == "" {
			value, err := promptSecret("Password: ")
			if err != nil {
				return err
			}
			password = value
		}

		if expiry == "" {
			expiry = "1_month"
		}
		if deviceName == "" {
			hostname, _ := os.Hostname()
			deviceName = "modelslab-cli@" + hostname
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/login", map[string]string{
			"email":        email,
			"password":     password,
			"expiry":       expiry,
			"token_expiry": expiry,
			"device_name":  deviceName,
		}, &result)
		if err != nil {
			apiErr, ok := err.(*api.APIError)
			if ok {
				output.PrintError(apiErr.Message, loginFailureHints(apiErr, email)...)
				os.Exit(apiErr.ExitCode)
			}
			return err
		}

		// Extract token (API returns "access_token" or "token")
		token := ""
		if data, ok := result["data"].(map[string]interface{}); ok {
			if t, ok := data["access_token"].(string); ok {
				token = t
			} else if t, ok := data["token"].(string); ok {
				token = t
			}
			// Also store API key if returned
			if k, ok := data["api_key"].(string); ok && k != "" {
				if err := auth.StoreAPIKey(flagProfile, k); err != nil {
					return fmt.Errorf("logged in, but could not store the API key: %w", err)
				}
			}
		} else if t, ok := result["token"].(string); ok {
			token = t
		} else if t, ok := result["access_token"].(string); ok {
			token = t
		}

		if token == "" {
			return fmt.Errorf("no token returned from login")
		}

		// Checked, not fire-and-forget. When both the keychain and the file
		// fallback fail — a read-only ~/.config, a denied keychain prompt — the
		// CLI used to print "Logged in" and exit 0 while storing nothing, and
		// every later command 401'd for no visible reason.
		if err := auth.StoreToken(flagProfile, token); err != nil {
			return fmt.Errorf("logged in, but could not store the access token: %w", err)
		}
		if err := auth.StoreEmail(flagProfile, email); err != nil {
			return fmt.Errorf("logged in, but could not store the account email: %w", err)
		}
		apiClient = nil

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Logged in as %s (profile: %s)", email, flagProfile))
			fmt.Printf("Token: %s\n", output.MaskSecret(token))
		})
		return nil
	},
}

// loginFailureHints turns a control-plane error code into next steps a user can
// actually act on.
//
// The old blanket "check your email and password" was wrong for the two failures
// people actually hit. An account created through Google or GitHub is stored with
// a random password nobody ever sees, so email+password login can never succeed
// for it and the only way in is `--browser`. And an unverified account fails with
// credentials that are perfectly correct.
func loginFailureHints(apiErr *api.APIError, email string) []string {
	switch apiErr.Code {
	case "email_not_verified":
		hint := "Run: modelslab auth resend-verification"
		if email != "" {
			hint += " --email " + email
		}
		return []string{"This account exists but its email is not verified yet.", hint}
	case "access_denied":
		return []string{"Contact support@modelslab.com if you think this is a mistake."}
	case "validation_error":
		// Say which field the server rejected instead of guessing at --email.
		if fields := apiErr.FieldErrors(); len(fields) > 0 {
			return append([]string{"The server rejected these values:"}, fields...)
		}
		return []string{"The server rejected the login payload — check --email and --expiry."}
	case "invalid_credentials":
		return []string{
			"Check your email and password.",
			"Signed up with Google or GitHub? That account has no password — run: modelslab auth login --browser",
			"Otherwise run: modelslab auth forgot-password",
		}
	case "":
		// No code means we could not read the response at all — a proxy error
		// page, a captive portal, an outage. Saying "check your password" here
		// sends people to reset a password that was never the problem.
		return []string{
			fmt.Sprintf("The server did not return a recognisable error (HTTP %d).", apiErr.StatusCode),
			"This is usually a network or service problem, not your credentials. Try again shortly.",
			"Check https://modelslab.com/status, or pass --base-url if you are pointing at a non-default host.",
		}
	default:
		return []string{"Run: modelslab auth login --browser", "Or run: modelslab auth forgot-password"}
	}
}

func runBrowserLogin(cmd *cobra.Command) error {
	expiry, _ := cmd.Flags().GetString("expiry")
	deviceName, _ := cmd.Flags().GetString("device-name")
	callbackPort, _ := cmd.Flags().GetInt("callback-port")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	noOpen, _ := cmd.Flags().GetBool("no-open")

	if expiry == "" {
		expiry = "1_month"
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if callbackPort < 0 || callbackPort > 65535 {
		return fmt.Errorf("--callback-port must be between 0 and 65535")
	}
	if deviceName == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "localhost"
		}
		deviceName = "modelslab-cli@" + hostname
	}

	state, err := randomOAuthState()
	if err != nil {
		return err
	}

	listenAddr := "127.0.0.1:0"
	if callbackPort > 0 {
		listenAddr = "127.0.0.1:" + strconv.Itoa(callbackPort)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("could not start local OAuth callback server: %w", err)
	}

	callbackURL := "http://" + listener.Addr().String() + "/callback"
	resultCh := make(chan browserLoginCallback, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", browserLoginCallbackHandler(state, resultCh))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErrCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	loginURL, err := buildBrowserLoginURL(flagBaseURL, callbackURL, state, deviceName, expiry)
	if err != nil {
		return err
	}

	/*
	 * The URL is printed unconditionally, before any attempt to open it.
	 *
	 * openBrowser uses exec.Start(), which returns nil the moment the child is
	 * spawned — so `xdg-open` with no display, or a Chrome that dies on launch,
	 * both looked like success. The user saw "Waiting for browser
	 * authorization..." and then a timeout five minutes later, and was never
	 * once shown the URL they could have pasted somewhere that works.
	 */
	fmt.Fprintf(os.Stderr, "Authorize ModelsLab CLI at:\n%s\n\n", loginURL)

	if !noOpen {
		fmt.Fprintln(os.Stderr, "Opening your browser...")
		if err := openBrowser(loginURL); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser automatically (%v) — open the URL above yourself.\n", err)
		}
	}
	fmt.Fprintln(os.Stderr, "Waiting for browser authorization...")

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var callback browserLoginCallback
	select {
	case callback = <-resultCh:
	case err := <-serveErrCh:
		return fmt.Errorf("OAuth callback server failed: %w", err)
	case <-timer.C:
		return fmt.Errorf(
			"browser login timed out after %s.\n"+
				"  Open the URL above and finish the grant there. If the browser sent you to the model\n"+
				"  catalogue instead of a grant page, sign in at https://modelslab.com/login first and retry.\n"+
				"  Raise the wait with --timeout, or print the URL without opening a browser with --no-open",
			timeout,
		)
	}

	if callback.Error != "" {
		return fmt.Errorf("browser login failed: %s", callback.Error)
	}
	if callback.State != state {
		return fmt.Errorf("browser login returned an invalid state")
	}
	if callback.APIKey == "" {
		return fmt.Errorf("browser login did not return an API key")
	}

	if callback.AccessToken != "" {
		if err := auth.StoreToken(flagProfile, callback.AccessToken); err != nil {
			return fmt.Errorf("could not store access token: %w", err)
		}
	}
	if err := auth.StoreAPIKey(flagProfile, callback.APIKey); err != nil {
		return fmt.Errorf("could not store API key: %w", err)
	}
	if callback.Email != "" {
		if err := auth.StoreEmail(flagProfile, callback.Email); err != nil {
			return fmt.Errorf("could not store email: %w", err)
		}
	}
	apiClient = nil

	data := map[string]interface{}{
		"access_token":           callback.AccessToken,
		"api_key":                callback.APIKey,
		"email":                  callback.Email,
		"expires_at":             callback.ExpiresAt,
		"message":                "Browser login successful.",
		"model_id":               callback.ModelID,
		"token_expiry":           callback.TokenExpiry,
		"token_expiry_effective": callback.TokenExpiryEffective,
		"token_lifetime_capped":  parseCallbackBool(callback.TokenLifetimeCapped),
		"token_type":             firstNonEmpty(callback.TokenType, "Bearer"),
	}
	result := map[string]interface{}{
		"data":  data,
		"error": nil,
	}

	outputResult(result, func() {
		output.PrintSuccess(fmt.Sprintf("Logged in with browser OAuth (profile: %s)", flagProfile))
		if callback.Email != "" {
			fmt.Printf("Email: %s\n", callback.Email)
		}
		if callback.AccessToken != "" {
			fmt.Printf("Token: %s\n", output.MaskSecret(callback.AccessToken))
		}
		fmt.Printf("API Key: %s\n", output.MaskSecret(callback.APIKey))
	})

	return nil
}

func browserLoginCallbackHandler(expectedState string, resultCh chan<- browserLoginCallback) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid callback payload", http.StatusBadRequest)
			return
		}

		callback := browserLoginCallback{
			AccessToken:          r.FormValue("access_token"),
			APIKey:               r.FormValue("api_key"),
			Email:                r.FormValue("email"),
			Error:                r.FormValue("error"),
			ExpiresAt:            r.FormValue("expires_at"),
			ModelID:              r.FormValue("model_id"),
			State:                r.FormValue("state"),
			TokenExpiry:          r.FormValue("token_expiry"),
			TokenExpiryEffective: r.FormValue("token_expiry_effective"),
			TokenLifetimeCapped:  r.FormValue("token_lifetime_capped"),
			TokenType:            r.FormValue("token_type"),
		}

		if callback.State != expectedState {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		if callback.Error == "" && callback.APIKey == "" {
			http.Error(w, "Missing API key", http.StatusBadRequest)
			return
		}

		select {
		case resultCh <- callback:
		default:
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		title := "ModelsLab CLI login complete"
		if callback.Error != "" {
			title = "ModelsLab CLI login failed"
		}
		fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title></head><body><h1>%s</h1><p>You can close this browser tab and return to the terminal.</p></body></html>", html.EscapeString(title), html.EscapeString(title))
	}
}

func buildBrowserLoginURL(baseURL, callbackURL, state, deviceName, tokenExpiry string) (string, error) {
	authorizeURL, err := url.Parse(strings.TrimRight(baseURL, "/") + "/auth/modelslab-cli/oauth/authorize")
	if err != nil {
		return "", fmt.Errorf("could not build browser login URL: %w", err)
	}

	query := authorizeURL.Query()
	query.Set("client_name", "ModelsLab CLI")
	query.Set("device_name", deviceName)
	query.Set("redirect_uri", callbackURL)
	query.Set("state", state)
	query.Set("token_expiry", tokenExpiry)
	authorizeURL.RawQuery = query.Encode()

	return authorizeURL.String(), nil
}

func randomOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("could not generate OAuth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func openBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		if err := exec.Command("open", "-a", "Google Chrome", rawURL).Run(); err == nil {
			return nil
		}
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

func parseCallbackBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// --- auth signup ---
var authSignupCmd = &cobra.Command{
	Use:   "signup",
	Short: "Create a new ModelsLab account",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		name, _ := cmd.Flags().GetString("name")

		if email == "" {
			value, err := promptLine("Email: ")
			if err != nil {
				return err
			}
			email = value
		}
		if password == "" {
			value, err := promptSecret("Password: ")
			if err != nil {
				return err
			}
			password = value
		}
		if name == "" {
			value, err := promptLine("Name: ")
			if err != nil {
				return err
			}
			name = value
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/signup", map[string]interface{}{
			"email":                 email,
			"password":              password,
			"password_confirmation": password,
			"name":                  name,
		}, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Account created! Check your email for verification.")
		})
		return nil
	},
}

// --- auth logout ---
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout current session",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/logout", nil, &result)
		if err != nil {
			// Still remove local credentials even if API call fails
		}
		auth.DeleteProfile(flagProfile)

		outputResult(result, func() {
			output.PrintSuccess("Logged out successfully.")
		})
		return nil
	},
}

// --- auth logout-all ---
var authLogoutAllCmd = &cobra.Command{
	Use:   "logout-all",
	Short: "Revoke all tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/logout-all", nil, &result)
		if err != nil {
			return err
		}
		auth.DeleteProfile(flagProfile)

		outputResult(result, func() {
			output.PrintSuccess("All sessions revoked.")
		})
		return nil
	},
}

// --- auth status ---
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := flagProfile
		email, _ := auth.GetEmail(profile)
		token, tokenErr := auth.GetToken(profile)
		apiKey, _ := auth.GetAPIKey(profile)

		data := map[string]interface{}{
			"profile":       profile,
			"email":         email,
			"authenticated": tokenErr == nil,
			"has_api_key":   apiKey != "",
		}

		outputResult(data, func() {
			pairs := [][2]string{
				{"Profile", profile},
				{"Email", email},
			}
			if tokenErr == nil {
				pairs = append(pairs, [2]string{"Token", output.MaskSecret(token)})
			} else {
				pairs = append(pairs, [2]string{"Token", "Not set"})
			}
			if apiKey != "" {
				pairs = append(pairs, [2]string{"API Key", output.MaskSecret(apiKey)})
			} else {
				pairs = append(pairs, [2]string{"API Key", "Not set"})
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

// --- auth forgot-password ---
var authForgotPasswordCmd = &cobra.Command{
	Use:   "forgot-password",
	Short: "Send password reset email",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			value, err := promptLine("Email: ")
			if err != nil {
				return err
			}
			email = value
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/forgot-password", map[string]string{
			"email": email,
		}, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Password reset email sent to " + email)
		})
		return nil
	},
}

// --- auth reset-password ---
var authResetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "Reset password with token",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		token, _ := cmd.Flags().GetString("token")
		password, _ := cmd.Flags().GetString("password")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/reset-password", map[string]string{
			"email":                 email,
			"token":                 token,
			"password":              password,
			"password_confirmation": password,
		}, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Password reset successfully.")
		})
		return nil
	},
}

// --- auth resend-verification ---
var authResendVerificationCmd = &cobra.Command{
	Use:   "resend-verification",
	Short: "Resend email verification",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			value, err := promptLine("Email: ")
			if err != nil {
				return err
			}
			email = value
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/resend-verification", map[string]string{
			"email": email,
		}, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Verification email resent to " + email)
		})
		return nil
	},
}

// --- auth tokens ---
var authTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List all active tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/auth/tokens", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if data, ok := result["data"].([]interface{}); ok {
				headers := []string{"ID", "NAME", "LAST USED", "CREATED"}
				rows := [][]string{}
				for _, item := range data {
					if t, ok := item.(map[string]interface{}); ok {
						rows = append(rows, []string{
							fmt.Sprintf("%v", t["id"]),
							fmt.Sprintf("%v", t["name"]),
							fmt.Sprintf("%v", t["last_used_at"]),
							fmt.Sprintf("%v", t["created_at"]),
						})
					}
				}
				output.PrintTable(headers, rows)
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

// --- auth revoke-token ---
var authRevokeTokenCmd = &cobra.Command{
	Use:   "revoke-token",
	Short: "Revoke a specific token",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("DELETE", "/auth/tokens/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Token " + id + " revoked.")
		})
		return nil
	},
}

// --- auth revoke-others ---
var authRevokeOthersCmd = &cobra.Command{
	Use:   "revoke-others",
	Short: "Revoke all tokens except current",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/tokens/revoke-others", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("All other tokens revoked.")
		})
		return nil
	},
}

// --- auth switch-account ---
var authSwitchAccountCmd = &cobra.Command{
	Use:   "switch-account",
	Short: "Switch to team member context",
	RunE: func(cmd *cobra.Command, args []string) error {
		memberID, _ := cmd.Flags().GetString("member-id")

		body := map[string]interface{}{}
		if memberID != "" {
			body["team_member_id"] = memberID
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/auth/switch-account", body, &result)
		if err != nil {
			return err
		}

		// Update token if returned
		if data, ok := result["data"].(map[string]interface{}); ok {
			if t, ok := data["token"].(string); ok {
				auth.StoreToken(flagProfile, t)
			}
		}

		outputResult(result, func() {
			if memberID != "" {
				output.PrintSuccess("Switched to team member context: " + memberID)
			} else {
				output.PrintSuccess("Switched back to personal account.")
			}
		})
		return nil
	},
}

func init() {
	// auth login
	authLoginCmd.Flags().String("email", "", "Account email")
	authLoginCmd.Flags().String("password", "", "Account password")
	authLoginCmd.Flags().Bool("browser", false, "Log in with browser OAuth instead of email and password")
	authLoginCmd.Flags().Int("callback-port", 0, "Local callback port for browser OAuth (0 chooses a free port)")
	authLoginCmd.Flags().String("expiry", "1_month", "Token expiry: 1_week, 1_month, 3_months, 6_months, 1_year, never")
	authLoginCmd.Flags().String("device-name", "", "Device name for token")
	authLoginCmd.Flags().Bool("no-open", false, "Print the browser login URL without opening Chrome")
	authLoginCmd.Flags().Duration("timeout", 5*time.Minute, "Browser OAuth timeout")

	// auth signup
	authSignupCmd.Flags().String("email", "", "Account email")
	authSignupCmd.Flags().String("password", "", "Account password")
	authSignupCmd.Flags().String("name", "", "Display name")

	// auth forgot-password
	authForgotPasswordCmd.Flags().String("email", "", "Account email")

	// auth reset-password
	authResetPasswordCmd.Flags().String("email", "", "Account email")
	authResetPasswordCmd.Flags().String("token", "", "Reset token from email")
	authResetPasswordCmd.Flags().String("password", "", "New password")
	authResetPasswordCmd.MarkFlagRequired("email")
	authResetPasswordCmd.MarkFlagRequired("token")
	authResetPasswordCmd.MarkFlagRequired("password")

	// auth resend-verification
	authResendVerificationCmd.Flags().String("email", "", "Account email")

	// auth revoke-token
	authRevokeTokenCmd.Flags().String("id", "", "Token ID to revoke")

	// auth switch-account
	authSwitchAccountCmd.Flags().String("member-id", "", "Team member ID to switch to (empty to switch back)")

	_ = strings.Join // suppress unused import

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authSignupCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authLogoutAllCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authForgotPasswordCmd)
	authCmd.AddCommand(authResetPasswordCmd)
	authCmd.AddCommand(authResendVerificationCmd)
	authCmd.AddCommand(authTokensCmd)
	authCmd.AddCommand(authRevokeTokenCmd)
	authCmd.AddCommand(authRevokeOthersCmd)
	authCmd.AddCommand(authSwitchAccountCmd)
}
