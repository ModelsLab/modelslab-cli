# ModelsLab CLI

AI generation and account management from the terminal. One command surface over
the ModelsLab API: image, video, audio, 3D and LLM generation, plus authentication,
billing, wallet, subscriptions and model discovery.

```bash
npm install -g modelslab-cli
modelslab auth login
modelslab generate image --prompt "a lighthouse at dusk" --model flux
```

The package installs a prebuilt binary for your platform as an optional
dependency — nothing is compiled and no install script runs.

Supported: macOS (Intel, Apple Silicon), Linux (x64, arm64), Windows (x64, arm64).

- Docs: https://docs.modelslab.com
- Source: https://github.com/ModelsLab/modelslab-cli
- Other install methods (Homebrew, Scoop, shell): https://modelslab.sh
