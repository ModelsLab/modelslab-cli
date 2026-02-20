package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ModelsLab/modelslab-cli/internal/auth"
	"github.com/ModelsLab/modelslab-cli/internal/config"
	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		// Special handling for api_key - store in keyring
		if key == "api_key" || key == "apikey" {
			if err := auth.StoreAPIKey(flagProfile, value); err != nil {
				return fmt.Errorf("could not store API key: %w", err)
			}
			output.PrintSuccess("API key stored securely.")
			return nil
		}

		if err := config.Set(key, value); err != nil {
			return err
		}
		output.PrintSuccess(fmt.Sprintf("Set %s = %s", key, value))
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := config.Get(key)
		if value == "" {
			return fmt.Errorf("key %q not set", key)
		}
		fmt.Println(value)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all config",
	RunE: func(cmd *cobra.Command, args []string) error {
		settings := config.AllSettings()

		outputResult(settings, func() {
			pairs := [][2]string{}
			flattenMap("", settings, &pairs)
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

func flattenMap(prefix string, m map[string]interface{}, pairs *[][2]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if sub, ok := v.(map[string]interface{}); ok {
			flattenMap(key, sub, pairs)
		} else {
			*pairs = append(*pairs, [2]string{key, fmt.Sprintf("%v", v)})
		}
	}
}

var configProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage auth profiles",
}

var configProfilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List auth profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		profilesDir := filepath.Join(home, ".config", "modelslab", "profiles")

		entries, err := os.ReadDir(profilesDir)
		if err != nil {
			fmt.Println("No profiles found.")
			return nil
		}

		headers := []string{"PROFILE", "EMAIL", "ACTIVE"}
		rows := [][]string{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if filepath.Ext(name) == ".json" {
				profile := name[:len(name)-5]
				email, _ := auth.GetEmail(profile)
				active := ""
				if profile == flagProfile {
					active = "●"
				}
				rows = append(rows, []string{profile, email, active})
			}
		}

		if len(rows) == 0 {
			// Check if default profile exists in keychain
			if email, err := auth.GetEmail("default"); err == nil {
				rows = append(rows, []string{"default", email, "●"})
			}
		}

		if len(rows) == 0 {
			fmt.Println("No profiles found. Run: modelslab auth login")
			return nil
		}

		output.PrintTable(headers, rows)
		return nil
	},
}

var configProfilesUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := config.Set("defaults.profile", name); err != nil {
			return err
		}
		output.PrintSuccess("Switched to profile: " + name)
		return nil
	},
}

var configProfilesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "default" {
			return fmt.Errorf("cannot delete the default profile")
		}
		auth.DeleteProfile(name)
		output.PrintSuccess("Profile " + name + " deleted.")
		return nil
	},
}

func init() {
	configProfilesCmd.AddCommand(configProfilesListCmd)
	configProfilesCmd.AddCommand(configProfilesUseCmd)
	configProfilesCmd.AddCommand(configProfilesDeleteCmd)

	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configProfilesCmd)
}
