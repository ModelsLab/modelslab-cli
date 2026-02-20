# ModelsLab CLI

The official command-line interface for [ModelsLab](https://modelslab.com) — manage accounts, discover models, generate AI content (image/video/audio/3D/chat), handle billing, and interact with the full platform from the terminal.

**Agent-first design** — every command works for both humans (colored tables, prompts) and AI agents (`--output json`, `--jq` filtering, semantic exit codes).

## Installation

### macOS (Homebrew)

```bash
brew install modelslab/tap/modelslab
```

### Windows (Scoop)

```powershell
scoop bucket add modelslab https://github.com/ModelsLab/scoop-bucket
scoop install modelslab
```

### Linux (apt/deb)

```bash
# Download the latest .deb from GitHub Releases
curl -fsSL https://github.com/ModelsLab/modelslab-cli/releases/latest/download/modelslab_linux_amd64.deb -o modelslab.deb
sudo dpkg -i modelslab.deb
```

### Direct Download (any platform)

```bash
curl -fsSL https://raw.githubusercontent.com/ModelsLab/modelslab-cli/main/install.sh | sh
```

### Go Install

```bash
go install github.com/ModelsLab/modelslab-cli/cmd/modelslab@latest
```

### Manual Download

Download the latest binary for your platform from [GitHub Releases](https://github.com/ModelsLab/modelslab-cli/releases).

| Platform | Architecture | Download |
|----------|-------------|----------|
| macOS | Apple Silicon (M1/M2/M3) | `modelslab_darwin_arm64.tar.gz` |
| macOS | Intel | `modelslab_darwin_amd64.tar.gz` |
| Linux | x86_64 | `modelslab_linux_amd64.tar.gz` |
| Linux | ARM64 | `modelslab_linux_arm64.tar.gz` |
| Windows | x86_64 | `modelslab_windows_amd64.zip` |

## Quick Start

```bash
# Login to your account
modelslab auth login

# Check your profile
modelslab profile get

# Search models
modelslab models search --search "flux" --per-page 5

# Generate an image
modelslab generate image --prompt "sunset over mountains" --model sdxl

# Check wallet balance
modelslab wallet balance

# Get JSON output
modelslab profile get --output json

# Filter with jq
modelslab models search --search "flux" --output json --jq '.[].model_id'
```

## Commands

```
modelslab auth          Manage authentication (login, signup, logout, tokens)
modelslab profile       Manage your profile (get, update, password, socials)
modelslab keys          Manage API keys (list, create, get, update, delete)
modelslab models        Discover models (search, detail, filters, tags, providers)
modelslab generate      Generate AI content (image, video, audio, 3D, chat)
modelslab billing       Manage billing (overview, payment methods, invoices)
modelslab wallet        Manage wallet (balance, fund, transactions, coupons)
modelslab subscriptions Manage subscriptions (plans, create, pause, resume)
modelslab teams         Manage teams (list, invite, update, remove)
modelslab usage         View usage analytics (summary, products, history)
modelslab config        Manage CLI configuration (set, get, profiles)
modelslab mcp           MCP server mode (serve, tools)
modelslab docs          Access API documentation (openapi, changelog)
modelslab completion    Generate shell completions (bash, zsh, fish, powershell)
```

## Authentication

The CLI uses two credential types:

- **Bearer token** — for control plane commands (auth, profile, billing, etc.)
- **API key** — for generation commands (image, video, audio, etc.)

```bash
# Login gets both token and API key
modelslab auth login --email you@example.com --password "..."

# Or set API key manually
modelslab config set api_key "your-api-key"

# Check auth status
modelslab auth status

# Use environment variables
export MODELSLAB_API_KEY="your-api-key"
export MODELSLAB_TOKEN="your-bearer-token"
```

## Output Modes

```bash
# Human mode (default) — colored tables
modelslab models search --search "flux"

# JSON mode
modelslab models search --search "flux" --output json

# JSON + jq filtering
modelslab models search --search "flux" --output json --jq '.[].model_id'
```

## Generation

```bash
# Text to image
modelslab generate image --prompt "sunset" --model sdxl

# Image to image
modelslab generate image-to-image --prompt "oil painting" --init-image https://...

# Text to video
modelslab generate video --prompt "ocean waves"

# Text to speech
modelslab generate tts --text "Hello world" --language en

# Chat completion
modelslab generate chat --message "Explain quantum computing" --model gpt-4

# Check generation status
modelslab generate fetch --id 12345 --type image

# Skip polling (async)
modelslab generate image --prompt "sunset" --no-wait
```

## MCP Server Mode

Use the CLI as an MCP server for AI assistants:

```bash
# Start MCP server (stdio)
modelslab mcp serve

# List available tools
modelslab mcp tools
```

**Claude Desktop / Claude Code config:**
```json
{
  "mcpServers": {
    "modelslab": {
      "command": "modelslab",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Billing & Payments

```bash
# View billing overview
modelslab billing overview

# Add funds to wallet
modelslab wallet fund --amount 25

# Add a payment method (card tokenized via Stripe)
modelslab billing add-payment-method --card-number 4242424242424242 --exp-month 12 --exp-year 2027 --cvc 123

# Subscribe to a plan
modelslab subscriptions plans
modelslab subscriptions create --plan-id 10 --payment-method pm_...

# Redeem a coupon
modelslab wallet redeem-coupon --code WELCOME50
```

## Configuration

```bash
# Set config values
modelslab config set defaults.output json
modelslab config set generation.default_model sdxl

# View all config
modelslab config list

# Manage profiles
modelslab config profiles list
modelslab config profiles use work
```

Config file: `~/.config/modelslab/config.toml`

## Shell Completions

```bash
# Bash
echo 'source <(modelslab completion bash)' >> ~/.bashrc

# Zsh
echo 'eval "$(modelslab completion zsh)"' >> ~/.zshrc

# Fish
modelslab completion fish > ~/.config/fish/completions/modelslab.fish

# PowerShell
modelslab completion powershell | Out-String | Invoke-Expression
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Usage error |
| `3` | Authentication error |
| `4` | Rate limited |
| `5` | Not found |
| `6` | Payment/billing error |
| `7` | Generation timeout |
| `10` | Network error |

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `MODELSLAB_API_KEY` | API key for generation endpoints |
| `MODELSLAB_TOKEN` | Bearer token for control plane |
| `MODELSLAB_BASE_URL` | Override base URL |
| `MODELSLAB_PROFILE` | Active profile name |
| `MODELSLAB_OUTPUT` | Default output format |
| `NO_COLOR` | Disable colored output |

## Development

```bash
# Build
go build -o modelslab ./cmd/modelslab/

# Run tests
go test ./internal/... -v
go test ./tests/ -v  # integration tests

# Run with local server
./modelslab --base-url http://127.0.0.1:8888 auth login

# Release (with GoReleaser)
goreleaser release --snapshot --clean
```

## License

MIT
