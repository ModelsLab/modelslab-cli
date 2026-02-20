package cmd

import (
	"fmt"

	"github.com/ModelsLab/cli/internal/output"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Access API documentation",
}

var docsOpenAPICmd = &cobra.Command{
	Use:   "openapi",
	Short: "Fetch and display OpenAPI spec",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/openapi.json", nil, &result)
		if err != nil {
			return err
		}

		output.PrintJSON(result)
		return nil
	},
}

var docsChangelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Show API changelog",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/changelog", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if entries, ok := result["data"].([]interface{}); ok {
				for _, entry := range entries {
					if e, ok := entry.(map[string]interface{}); ok {
						fmt.Printf("[%v] %v\n", e["date"], e["title"])
						if desc, ok := e["description"].(string); ok {
							fmt.Printf("  %s\n\n", desc)
						}
					}
				}
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

func init() {
	docsCmd.AddCommand(docsOpenAPICmd)
	docsCmd.AddCommand(docsChangelogCmd)
}
