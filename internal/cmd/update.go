package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ModelsLab/modelslab-cli/internal/config"
	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/ModelsLab/modelslab-cli/internal/updater"
	"github.com/spf13/cobra"
)

var (
	updateCheckOnly    bool
	updateForce        bool
	updateSkipChecksum bool
	updateRepo         string
	updateTimeout      time.Duration
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the ModelsLab CLI",
	Long: `Check GitHub Releases for a newer ModelsLab CLI version and install it.

The updater downloads the release archive for your platform, verifies it against
the release checksums.txt file, and replaces the current executable.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if updateTimeout <= 0 {
			updateTimeout = 2 * time.Minute
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), updateTimeout)
		defer cancel()

		options := updater.InstallOptions{
			CheckOptions: updater.CheckOptions{
				CurrentVersion: cliVersion,
				Repo:           firstNonEmpty(updateRepo, config.Get("updates.github_repo"), updater.DefaultRepo),
				HTTPClient: &http.Client{
					Timeout: updateTimeout,
				},
				UserAgent: "modelslab-cli/" + cliVersion,
			},
			Force:        updateForce,
			SkipChecksum: updateSkipChecksum,
		}

		if updateCheckOnly {
			info, err := updater.Check(ctx, options.CheckOptions)
			if err != nil {
				return err
			}
			outputResult(info, func() {
				printUpdateCheck(info)
			})
			return nil
		}

		result, err := updater.Install(ctx, options)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			printUpdateInstallResult(result)
		})

		return nil
	},
}

func printUpdateCheck(info *updater.Info) {
	if !info.CanCompare {
		fmt.Printf("Latest ModelsLab CLI release: %s\n", formatVersion(info.LatestVersion))
		fmt.Printf("Current build: %s\n", info.CurrentVersion)
		fmt.Println("This build version cannot be compared automatically.")
		fmt.Println("Run `modelslab update --force` to install the latest release over this binary.")
		return
	}

	if info.UpdateAvailable {
		fmt.Printf("Update available: %s -> %s\n", formatVersion(info.CurrentVersion), formatVersion(info.LatestVersion))
		fmt.Println("Run `modelslab update` to install it.")
		return
	}

	fmt.Printf("ModelsLab CLI is up to date (%s).\n", formatVersion(info.CurrentVersion))
}

func printUpdateInstallResult(result *updater.InstallResult) {
	if !result.UpdateAvailable && !updateForce {
		fmt.Printf("ModelsLab CLI is up to date (%s).\n", formatVersion(result.CurrentVersion))
		return
	}

	output.PrintSuccess(fmt.Sprintf("Updated ModelsLab CLI to %s.", formatVersion(result.LatestVersion)))
	if result.ChecksumVerified {
		fmt.Println("Checksum verified.")
	}
	if result.InstalledPath != "" {
		fmt.Println("Installed:", result.InstalledPath)
	}
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Only check whether an update is available")
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "Install the latest release even if the current version cannot be compared")
	updateCmd.Flags().BoolVar(&updateSkipChecksum, "skip-checksum", false, "Skip release checksum verification")
	updateCmd.Flags().StringVar(&updateRepo, "repo", "", "GitHub repo to check (default \"ModelsLab/modelslab-cli\")")
	updateCmd.Flags().DurationVar(&updateTimeout, "timeout", 2*time.Minute, "Update check/download timeout")
}
