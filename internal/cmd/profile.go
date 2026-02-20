package cmd

import (
	"fmt"

	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage your profile",
}

var profileGetCmd = &cobra.Command{
	Use:   "get",
	Short: "View your profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/me", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for _, key := range []string{"name", "username", "email", "wallet_balance", "about", "created_at"} {
				if v, ok := data[key]; ok && v != nil {
					pairs = append(pairs, [2]string{key, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var profileUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update profile info",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("name"); v != "" {
			body["name"] = v
		}
		if v, _ := cmd.Flags().GetString("username"); v != "" {
			body["username"] = v
		}
		if v, _ := cmd.Flags().GetString("about"); v != "" {
			body["about"] = v
		}

		if len(body) == 0 {
			return fmt.Errorf("at least one field must be specified: --name, --username, --about")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("PATCH", "/me", body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Profile updated.")
		})
		return nil
	},
}

var profileUpdatePasswordCmd = &cobra.Command{
	Use:   "update-password",
	Short: "Change your password",
	RunE: func(cmd *cobra.Command, args []string) error {
		current, _ := cmd.Flags().GetString("current-password")
		newPw, _ := cmd.Flags().GetString("new-password")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("PATCH", "/me/password", map[string]string{
			"current_password":      current,
			"password":              newPw,
			"password_confirmation": newPw,
		}, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Password updated.")
		})
		return nil
	},
}

var profileUpdateSocialsCmd = &cobra.Command{
	Use:   "update-socials",
	Short: "Update social links",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("twitter"); v != "" {
			body["twitter"] = v
		}
		if v, _ := cmd.Flags().GetString("github"); v != "" {
			body["github"] = v
		}
		if v, _ := cmd.Flags().GetString("website"); v != "" {
			body["website"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("PATCH", "/me/socials", body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Social links updated.")
		})
		return nil
	},
}

var profileUpdatePreferencesCmd = &cobra.Command{
	Use:   "update-preferences",
	Short: "Update notification and content preferences",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{}
		if cmd.Flags().Changed("nsfw") {
			v, _ := cmd.Flags().GetBool("nsfw")
			body["nsfw_enabled"] = v
		}
		if cmd.Flags().Changed("notifications") {
			v, _ := cmd.Flags().GetBool("notifications")
			body["notifications_enabled"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("PATCH", "/me/preferences", body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Preferences updated.")
		})
		return nil
	},
}

func init() {
	profileUpdateCmd.Flags().String("name", "", "Display name")
	profileUpdateCmd.Flags().String("username", "", "Username")
	profileUpdateCmd.Flags().String("about", "", "About text")

	profileUpdatePasswordCmd.Flags().String("current-password", "", "Current password")
	profileUpdatePasswordCmd.Flags().String("new-password", "", "New password")
	profileUpdatePasswordCmd.MarkFlagRequired("current-password")
	profileUpdatePasswordCmd.MarkFlagRequired("new-password")

	profileUpdateSocialsCmd.Flags().String("twitter", "", "Twitter URL")
	profileUpdateSocialsCmd.Flags().String("github", "", "GitHub URL")
	profileUpdateSocialsCmd.Flags().String("website", "", "Website URL")

	profileUpdatePreferencesCmd.Flags().Bool("nsfw", false, "Enable NSFW content")
	profileUpdatePreferencesCmd.Flags().Bool("notifications", false, "Enable notifications")

	profileCmd.AddCommand(profileGetCmd)
	profileCmd.AddCommand(profileUpdateCmd)
	profileCmd.AddCommand(profileUpdatePasswordCmd)
	profileCmd.AddCommand(profileUpdateSocialsCmd)
	profileCmd.AddCommand(profileUpdatePreferencesCmd)
}
