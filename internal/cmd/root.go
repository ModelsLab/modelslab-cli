package cmd

import (
	"os"

	"github.com/ModelsLab/cli/internal/api"
	"github.com/ModelsLab/cli/internal/auth"
	"github.com/ModelsLab/cli/internal/config"
	"github.com/ModelsLab/cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	cliVersion = "dev"
	cliCommit  = "none"
	cliDate    = "unknown"

	// Global flags
	flagOutput  string
	flagProfile string
	flagBaseURL string
	flagAPIKey  string
	flagJQ      string
	flagNoColor bool

	// Shared API client
	apiClient *api.Client
)

func SetVersion(version, commit, date string) {
	cliVersion = version
	cliCommit = commit
	cliDate = date
}

var rootCmd = &cobra.Command{
	Use:   "modelslab",
	Short: "ModelsLab CLI — AI generation and account management",
	Long: `modelslab is the official CLI for the ModelsLab platform.

Manage accounts, discover models, generate AI content (image/video/audio/3D/chat),
handle billing, and interact with the full platform from the terminal.

Designed for both humans and AI agents.`,
	Version: cliVersion,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "__complete" {
			return nil
		}

		if err := config.Init(); err != nil {
			return err
		}

		if flagOutput == "" {
			flagOutput = config.GetOutput()
		}
		if flagProfile == "" {
			flagProfile = config.GetProfile()
		}
		if flagBaseURL == "" {
			flagBaseURL = config.GetBaseURL()
		}
		if flagAPIKey == "" {
			flagAPIKey = config.GetAPIKey()
		}

		if os.Getenv("NO_COLOR") != "" {
			flagNoColor = true
		}

		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format: human, json (default \"human\")")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Auth profile to use (default \"default\")")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "", "Override API base URL")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "Override API key")
	rootCmd.PersistentFlags().StringVar(&flagJQ, "jq", "", "Filter JSON output with jq expression")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(keysCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(billingCmd)
	rootCmd.AddCommand(walletCmd)
	rootCmd.AddCommand(subscriptionsCmd)
	rootCmd.AddCommand(teamsCmd)
	rootCmd.AddCommand(usageCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(completionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func getClient() *api.Client {
	if apiClient != nil {
		return apiClient
	}

	token := ""
	apiKey := flagAPIKey

	if t, err := auth.GetToken(flagProfile); err == nil {
		token = t
	}
	if apiKey == "" {
		if k, err := auth.GetAPIKey(flagProfile); err == nil {
			apiKey = k
		}
	}

	if envToken := os.Getenv("MODELSLAB_TOKEN"); envToken != "" {
		token = envToken
	}

	apiClient = api.NewClient(flagBaseURL, token, apiKey)
	apiClient.SetVersion(cliVersion)
	return apiClient
}

// outputResult handles output based on --output and --jq flags.
func outputResult(data interface{}, humanFn func()) {
	if flagJQ != "" {
		output.PrintJQ(data, flagJQ)
		return
	}

	switch flagOutput {
	case "json":
		output.PrintJSON(data)
	default:
		humanFn()
	}
}
