package cmd

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/ModelsLab/modelslab-cli/internal/auth"
	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Login, logout, and manage authentication tokens and profiles.",
}

// --- auth login ---
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to ModelsLab",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		expiry, _ := cmd.Flags().GetString("expiry")
		deviceName, _ := cmd.Flags().GetString("device-name")

		if email == "" {
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}
		if password == "" {
			fmt.Print("Password: ")
			bytePw, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("could not read password: %w", err)
			}
			password = string(bytePw)
			fmt.Println()
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
			"email":       email,
			"password":    password,
			"expiry":      expiry,
			"device_name": deviceName,
		}, &result)
		if err != nil {
			apiErr, ok := err.(*api.APIError)
			if ok {
				output.PrintError(apiErr.Message, "Check your email and password.", "Run: modelslab auth forgot-password")
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
				auth.StoreAPIKey(flagProfile, k)
			}
		} else if t, ok := result["token"].(string); ok {
			token = t
		} else if t, ok := result["access_token"].(string); ok {
			token = t
		}

		if token == "" {
			return fmt.Errorf("no token returned from login")
		}

		// Store credentials
		auth.StoreToken(flagProfile, token)
		auth.StoreEmail(flagProfile, email)

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Logged in as %s (profile: %s)", email, flagProfile))
			fmt.Printf("Token: %s\n", output.MaskSecret(token))
		})
		return nil
	},
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
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}
		if password == "" {
			fmt.Print("Password: ")
			bytePw, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("could not read password: %w", err)
			}
			password = string(bytePw)
			fmt.Println()
		}
		if name == "" {
			fmt.Print("Name: ")
			fmt.Scanln(&name)
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
			"profile":        profile,
			"email":          email,
			"authenticated":  tokenErr == nil,
			"has_api_key":    apiKey != "",
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
			fmt.Print("Email: ")
			fmt.Scanln(&email)
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
			fmt.Print("Email: ")
			fmt.Scanln(&email)
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
	authLoginCmd.Flags().String("expiry", "1_month", "Token expiry: 1_week, 1_month, 3_months, 6_months, 1_year, never")
	authLoginCmd.Flags().String("device-name", "", "Device name for token")

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
