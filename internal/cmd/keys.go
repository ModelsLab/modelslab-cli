package cmd

import (
	"fmt"

	"github.com/ModelsLab/modelslab-cli/internal/auth"
	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/api-keys", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			items := extractItems(result)
			if items == nil {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "NAME", "KEY", "CREATED"}
			rows := [][]string{}
			for _, item := range items {
				if k, ok := item.(map[string]interface{}); ok {
					key := fmt.Sprintf("%v", k["key"])
					rows = append(rows, []string{
						fmt.Sprintf("%v", k["id"]),
						fmt.Sprintf("%v", k["name"]),
						output.MaskSecret(key),
						fmt.Sprintf("%v", k["created_at"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		notes, _ := cmd.Flags().GetString("notes")

		body := map[string]interface{}{}
		if name != "" {
			body["name"] = name
		}
		if notes != "" {
			body["notes"] = notes
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("POST", "/api-keys", body, &result)
		if err != nil {
			return err
		}

		// Auto-store the API key
		if data, ok := result["data"].(map[string]interface{}); ok {
			if key, ok := data["key"].(string); ok {
				auth.StoreAPIKey(flagProfile, key)
			}
		}

		outputResult(result, func() {
			if data, ok := result["data"].(map[string]interface{}); ok {
				output.PrintSuccess(fmt.Sprintf("API key created: %v", data["key"]))
				fmt.Println("Key stored in profile:", flagProfile)
			}
		})
		return nil
	},
}

var keysGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get API key details",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/api-keys/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for _, key := range []string{"id", "name", "key", "notes", "created_at"} {
				if v, ok := data[key]; ok && v != nil {
					val := fmt.Sprintf("%v", v)
					if key == "key" {
						val = output.MaskSecret(val)
					}
					pairs = append(pairs, [2]string{key, val})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var keysUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("name"); v != "" {
			body["name"] = v
		}
		if v, _ := cmd.Flags().GetString("notes"); v != "" {
			body["notes"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("PUT", "/api-keys/"+id, body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("API key updated.")
		})
		return nil
	},
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("DELETE", "/api-keys/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("API key " + id + " deleted.")
		})
		return nil
	},
}

func init() {
	keysCreateCmd.Flags().String("name", "", "Key name")
	keysCreateCmd.Flags().String("notes", "", "Key notes")

	keysGetCmd.Flags().String("id", "", "Key ID")
	keysUpdateCmd.Flags().String("id", "", "Key ID")
	keysUpdateCmd.Flags().String("name", "", "Key name")
	keysUpdateCmd.Flags().String("notes", "", "Key notes")
	keysDeleteCmd.Flags().String("id", "", "Key ID")

	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysCreateCmd)
	keysCmd.AddCommand(keysGetCmd)
	keysCmd.AddCommand(keysUpdateCmd)
	keysCmd.AddCommand(keysDeleteCmd)
}
