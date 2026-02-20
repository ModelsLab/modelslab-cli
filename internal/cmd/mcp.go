package cmd

import (
	"fmt"

	mcpserver "github.com/ModelsLab/modelslab-cli/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server mode",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server",
	Long:  "Start an MCP server exposing all ModelsLab tools for AI assistants.",
	RunE: func(cmd *cobra.Command, args []string) error {
		transport, _ := cmd.Flags().GetString("transport")

		client := getClient()
		server := mcpserver.NewServer(client)

		switch transport {
		case "stdio":
			return server.ServeStdio()
		case "sse":
			addr, _ := cmd.Flags().GetString("addr")
			return server.ServeSSE(addr)
		default:
			return fmt.Errorf("unknown transport %q, must be stdio or sse", transport)
		}
	},
}

var mcpToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List available MCP tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		server := mcpserver.NewServer(client)
		tools := server.ListTools()

		outputResult(tools, func() {
			for _, t := range tools {
				fmt.Printf("%-35s %s\n", t.Name, t.Description)
			}
		})
		return nil
	},
}

func init() {
	mcpServeCmd.Flags().String("transport", "stdio", "Transport: stdio, sse")
	mcpServeCmd.Flags().String("addr", ":8080", "SSE server address")

	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
}
