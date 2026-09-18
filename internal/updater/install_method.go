package updater

import (
	"os"
	"path/filepath"
	"strings"
)

// InstallMethod says who owns the running binary, and so who has to replace it.
type InstallMethod struct {
	// Name is the package manager that installed the binary, or "" when the
	// binary stands alone (install.sh, a release archive, go build).
	Name string `json:"name,omitempty"`
	// UpdateCommand is what the user should run to get the latest release.
	UpdateCommand string `json:"update_command"`
}

// Managed reports whether a package manager owns the binary.
//
// `modelslab update` must not replace a managed binary itself. It works, once,
// and then the package manager's record no longer matches the file on disk: pip
// still reports the old version, the next `brew upgrade` or `npm install` puts
// the old file back, and on Windows the pip and npm launchers hold the file
// open, so the replace fails outright.
func (m InstallMethod) Managed() bool {
	return m.Name != ""
}

var standalone = InstallMethod{UpdateCommand: "modelslab update"}

// CurrentInstallMethod detects how the running binary was installed.
func CurrentInstallMethod() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return standalone
	}
	// Homebrew runs the CLI through a symlink in bin/; the Caskroom path that
	// identifies it only shows up once the link is resolved.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	return DetectInstallMethod(exe)
}

// DetectInstallMethod maps an executable path to the package manager that owns
// it. It matches on the directory layout each one uses, which is stable, rather
// than asking the package managers, which may not be on PATH.
func DetectInstallMethod(exePath string) InstallMethod {
	// Lowercase and forward slashes, so one set of patterns covers Windows too.
	path := strings.ToLower(strings.ReplaceAll(exePath, `\`, "/"))

	switch {
	case strings.Contains(path, "/modelslab_cli/bin/") && strings.Contains(path, "/pipx/"):
		return InstallMethod{Name: "pipx", UpdateCommand: "pipx upgrade modelslab-cli"}
	case strings.Contains(path, "/modelslab_cli/bin/") && strings.Contains(path, "/uv/tools/"):
		return InstallMethod{Name: "uv", UpdateCommand: "uv tool upgrade modelslab-cli"}
	case strings.Contains(path, "/modelslab_cli/bin/"):
		return InstallMethod{Name: "pip", UpdateCommand: "pip install --upgrade modelslab-cli"}
	case strings.Contains(path, "/node_modules/@modelslab/cli-"):
		return InstallMethod{Name: "npm", UpdateCommand: "npm install -g modelslab-cli@latest"}
	case strings.Contains(path, "/caskroom/modelslab/"):
		return InstallMethod{Name: "homebrew", UpdateCommand: "brew upgrade modelslab/tap/modelslab"}
	case strings.Contains(path, "/scoop/apps/modelslab/"):
		return InstallMethod{Name: "scoop", UpdateCommand: "scoop update modelslab"}
	default:
		return standalone
	}
}
