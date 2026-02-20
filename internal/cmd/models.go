package cmd

import (
	"fmt"
	"strings"

	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Discover and search models",
}

var modelsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search 50,000+ models",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		// Build query params
		path := "/models?"
		params := []string{}
		if v, _ := cmd.Flags().GetString("search"); v != "" {
			params = append(params, "search="+v)
		}
		if v, _ := cmd.Flags().GetString("feature"); v != "" {
			params = append(params, "feature="+v)
		}
		if v, _ := cmd.Flags().GetString("provider"); v != "" {
			params = append(params, "provider="+v)
		}
		if v, _ := cmd.Flags().GetString("model-type"); v != "" {
			params = append(params, "model_type="+v)
		}
		if v, _ := cmd.Flags().GetString("model-subcategory"); v != "" {
			params = append(params, "model_subcategory="+v)
		}
		if v, _ := cmd.Flags().GetString("base-model"); v != "" {
			params = append(params, "base_model="+v)
		}
		if v, _ := cmd.Flags().GetString("tags"); v != "" {
			params = append(params, "tags="+v)
		}
		if v, _ := cmd.Flags().GetString("sort"); v != "" {
			params = append(params, "sort="+v)
		}
		if v, _ := cmd.Flags().GetInt("per-page"); v > 0 {
			params = append(params, fmt.Sprintf("per_page=%d", v))
		}
		if v, _ := cmd.Flags().GetInt("page"); v > 0 {
			params = append(params, fmt.Sprintf("page=%d", v))
		}
		path += strings.Join(params, "&")

		var result map[string]interface{}
		err := client.DoControlPlane("GET", path, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			items := extractItems(result)
			if items == nil {
				output.PrintJSON(result)
				return
			}
			headers := []string{"MODEL ID", "NAME", "PROVIDER", "TYPE"}
			rows := [][]string{}
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					name := firstNonNil(m, "model_name", "name")
					provider := firstNonNil(m, "provider", "model_category")
					mtype := firstNonNil(m, "model_type", "feature")
					rows = append(rows, []string{
						fmt.Sprintf("%v", m["model_id"]),
						truncate(name, 30),
						provider,
						mtype,
					})
				}
			}
			output.PrintTable(headers, rows)

			// Print total
			if meta, ok := result["meta"].(map[string]interface{}); ok {
				fmt.Printf("\n%d results", len(rows))
				if total, ok := meta["total"]; ok {
					fmt.Printf(" (%v total)", total)
				}
				fmt.Println()
			}
		})
		return nil
	},
}

var modelsDetailCmd = &cobra.Command{
	Use:   "detail",
	Short: "Get model details",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/models/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for _, key := range []string{"model_id", "name", "provider", "model_type", "model_subcategory", "base_model", "description"} {
				if v, ok := data[key]; ok && v != nil {
					pairs = append(pairs, [2]string{key, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var modelsFiltersCmd = &cobra.Command{
	Use:   "filters",
	Short: "List available filter values",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/models/filters", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintJSON(result)
		})
		return nil
	},
}

var modelsTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List available tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/models/tags", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if data, ok := result["data"].([]interface{}); ok {
				for _, tag := range data {
					fmt.Println(tag)
				}
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

var modelsProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List model providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/models/providers", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if data, ok := result["data"].([]interface{}); ok {
				for _, p := range data {
					fmt.Println(p)
				}
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	modelsSearchCmd.Flags().String("search", "", "Search query")
	modelsSearchCmd.Flags().String("feature", "", "Feature filter (imagen, video, audio, etc.)")
	modelsSearchCmd.Flags().String("provider", "", "Provider filter")
	modelsSearchCmd.Flags().String("model-type", "", "Model type filter")
	modelsSearchCmd.Flags().String("model-subcategory", "", "Model subcategory filter")
	modelsSearchCmd.Flags().String("base-model", "", "Base model filter")
	modelsSearchCmd.Flags().String("tags", "", "Tags filter (comma-separated)")
	modelsSearchCmd.Flags().String("sort", "", "Sort order")
	modelsSearchCmd.Flags().Int("per-page", 20, "Results per page")
	modelsSearchCmd.Flags().Int("page", 1, "Page number")

	modelsDetailCmd.Flags().String("id", "", "Model ID")

	modelsCmd.AddCommand(modelsSearchCmd)
	modelsCmd.AddCommand(modelsDetailCmd)
	modelsCmd.AddCommand(modelsFiltersCmd)
	modelsCmd.AddCommand(modelsTagsCmd)
	modelsCmd.AddCommand(modelsProvidersCmd)
}
