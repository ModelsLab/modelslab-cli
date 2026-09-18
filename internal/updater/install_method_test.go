package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectInstallMethod(t *testing.T) {
	tests := []struct {
		path    string
		name    string
		command string
	}{
		// pip on Windows, inside a venv: the setup from the support ticket.
		{`G:\ModelsLab\venv\Lib\site-packages\modelslab_cli\bin\modelslab.exe`, "pip", "pip install --upgrade modelslab-cli"},
		{"/usr/lib/python3/dist-packages/modelslab_cli/bin/modelslab", "pip", "pip install --upgrade modelslab-cli"},
		{"/Users/ada/.local/pipx/venvs/modelslab-cli/lib/python3.12/site-packages/modelslab_cli/bin/modelslab", "pipx", "pipx upgrade modelslab-cli"},
		{"/home/ada/.local/share/uv/tools/modelslab-cli/lib/python3.12/site-packages/modelslab_cli/bin/modelslab", "uv", "uv tool upgrade modelslab-cli"},
		{`C:\Users\Ada\AppData\Roaming\npm\node_modules\modelslab-cli\node_modules\@modelslab\cli-win32-x64\bin\modelslab.exe`, "npm", "npm install -g modelslab-cli@latest"},
		{"/Users/ada/proj/node_modules/.pnpm/@modelslab+cli-darwin-arm64@0.2.0/node_modules/@modelslab/cli-darwin-arm64/bin/modelslab", "npm", "npm install -g modelslab-cli@latest"},
		{"/opt/homebrew/Caskroom/modelslab/0.2.0/modelslab", "homebrew", "brew upgrade modelslab/tap/modelslab"},
		{`C:\Users\Ada\scoop\apps\modelslab\current\modelslab.exe`, "scoop", "scoop update modelslab"},
		{"/usr/local/bin/modelslab", "", "modelslab update"},
		{"/home/ada/.local/bin/modelslab", "", "modelslab update"},
		{`C:\tools\modelslab.exe`, "", "modelslab update"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			method := DetectInstallMethod(tt.path)

			assert.Equal(t, tt.name, method.Name)
			assert.Equal(t, tt.command, method.UpdateCommand)
			assert.Equal(t, tt.name != "", method.Managed())
		})
	}
}
