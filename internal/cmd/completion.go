package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completions",
	Long: `Generate shell completion scripts for modelslab CLI.

To load completions:

Bash:
  $ source <(modelslab completion bash)
  # To load completions for each session:
  $ echo 'source <(modelslab completion bash)' >> ~/.bashrc

Zsh:
  $ source <(modelslab completion zsh)
  # To load completions for each session:
  $ echo 'eval "$(modelslab completion zsh)"' >> ~/.zshrc

Fish:
  $ modelslab completion fish | source
  # To load completions for each session:
  $ modelslab completion fish > ~/.config/fish/completions/modelslab.fish

PowerShell:
  PS> modelslab completion powershell | Out-String | Invoke-Expression
  # To load completions for each session, add the output to your profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}
