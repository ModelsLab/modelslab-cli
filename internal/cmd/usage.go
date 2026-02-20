package cmd

import (
	"fmt"

	"github.com/ModelsLab/cli/internal/output"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "View usage analytics",
}

var usageSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Usage overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/usage/summary", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for k, v := range data {
				if v != nil {
					pairs = append(pairs, [2]string{k, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var usageProductsCmd = &cobra.Command{
	Use:   "products",
	Short: "Per-product breakdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/usage/products", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"PRODUCT", "REQUESTS", "COST"}
			rows := [][]string{}
			for _, item := range data {
				if p, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", p["product"]),
						fmt.Sprintf("%v", p["requests"]),
						fmt.Sprintf("$%v", p["cost"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var usageHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Generation history",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		path := "/usage/history"
		params := ""
		if v, _ := cmd.Flags().GetString("from"); v != "" {
			params += "from=" + v + "&"
		}
		if v, _ := cmd.Flags().GetString("to"); v != "" {
			params += "to=" + v + "&"
		}
		if v, _ := cmd.Flags().GetInt("per-page"); v > 0 {
			params += fmt.Sprintf("per_page=%d&", v)
		}
		if params != "" {
			path += "?" + params
		}

		var result map[string]interface{}
		err := client.DoControlPlane("GET", path, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "TYPE", "STATUS", "COST", "DATE"}
			rows := [][]string{}
			for _, item := range data {
				if h, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", h["id"]),
						fmt.Sprintf("%v", h["type"]),
						fmt.Sprintf("%v", h["status"]),
						fmt.Sprintf("$%v", h["cost"]),
						fmt.Sprintf("%v", h["created_at"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

func init() {
	usageHistoryCmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	usageHistoryCmd.Flags().String("to", "", "End date (YYYY-MM-DD)")
	usageHistoryCmd.Flags().Int("per-page", 20, "Results per page")

	usageCmd.AddCommand(usageSummaryCmd)
	usageCmd.AddCommand(usageProductsCmd)
	usageCmd.AddCommand(usageHistoryCmd)
}
