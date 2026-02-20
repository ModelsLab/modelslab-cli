package cmd

import (
	"fmt"

	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Manage team members",
}

var teamsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List team members",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/teams", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "NAME", "EMAIL", "ROLE", "STATUS"}
			rows := [][]string{}
			for _, item := range data {
				if m, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", m["id"]),
						fmt.Sprintf("%v", m["name"]),
						fmt.Sprintf("%v", m["email"]),
						fmt.Sprintf("%v", m["role"]),
						fmt.Sprintf("%v", m["status"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var teamsInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite a team member",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		role, _ := cmd.Flags().GetString("role")

		body := map[string]interface{}{
			"email": email,
		}
		if role != "" {
			body["role"] = role
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/teams", body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Invitation sent to " + email)
		})
		return nil
	},
}

var teamsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get team member details",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/teams/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintJSON(result)
		})
		return nil
	},
}

var teamsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update team member",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("role"); v != "" {
			body["role"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("PUT", "/teams/"+id, body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Team member updated.")
		})
		return nil
	},
}

var teamsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove team member",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("DELETE", "/teams/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Team member removed.")
		})
		return nil
	},
}

var teamsResendInviteCmd = &cobra.Command{
	Use:   "resend-invite",
	Short: "Resend invitation",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/teams/"+id+"/resend-invite", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Invitation resent.")
		})
		return nil
	},
}

var teamsAcceptInviteCmd = &cobra.Command{
	Use:   "accept-invite",
	Short: "Accept team invitation",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required (invitation UUID)")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/teams/invitations/"+id+"/accept", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Invitation accepted!")
		})
		return nil
	},
}

func init() {
	teamsInviteCmd.Flags().String("email", "", "Member email")
	teamsInviteCmd.Flags().String("role", "", "Member role")
	teamsInviteCmd.MarkFlagRequired("email")

	teamsGetCmd.Flags().String("id", "", "Member ID")
	teamsUpdateCmd.Flags().String("id", "", "Member ID")
	teamsUpdateCmd.Flags().String("role", "", "New role")
	teamsRemoveCmd.Flags().String("id", "", "Member ID")
	teamsResendInviteCmd.Flags().String("id", "", "Member ID")
	teamsAcceptInviteCmd.Flags().String("id", "", "Invitation UUID")

	teamsCmd.AddCommand(teamsListCmd)
	teamsCmd.AddCommand(teamsInviteCmd)
	teamsCmd.AddCommand(teamsGetCmd)
	teamsCmd.AddCommand(teamsUpdateCmd)
	teamsCmd.AddCommand(teamsRemoveCmd)
	teamsCmd.AddCommand(teamsResendInviteCmd)
	teamsCmd.AddCommand(teamsAcceptInviteCmd)
}
