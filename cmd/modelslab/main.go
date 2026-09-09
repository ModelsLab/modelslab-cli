package main

import (
	"os"

	"github.com/ModelsLab/modelslab-cli/internal/cmd"
)

// Set by goreleaser ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersion(version, commit, date)
	if err := cmd.Execute(); err != nil {
		os.Exit(cmd.ReportError(err))
	}
}
