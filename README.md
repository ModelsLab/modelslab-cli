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
curl -fsSL https://modelslab.sh/install.sh | sh
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
# New user? Sign up first
modelslab auth signup --name "Your Name" --email you@example.com --password "..." --confirm-password "..."

# Login to your account
modelslab auth login --email you@example.com --password "..."

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

### Signup & Onboarding

Agents can programmatically create accounts and manage the full auth lifecycle:

```bash
# 1. Create account
modelslab auth signup --name "Your Name" --email you@example.com --password "..." --confirm-password "..."

# 2. Verify email (token from verification email)
modelslab auth verify-email --token <verification-token>

# 3. Login (auto-stores bearer token + API key in OS keychain)
modelslab auth login --email you@example.com --password "..."

# 4. Check status
modelslab auth status

# Password recovery
modelslab auth forgot-password --email you@example.com
modelslab auth reset-password --token <reset-token> --password "..." --confirm-password "..."

# Token management
modelslab auth tokens list
modelslab auth tokens create --name "ci-token"
```

### Existing Users

```bash
# Login gets both token and API key
modelslab auth login --email you@example.com --password "..."

# Or set API key manually
modelslab config set api_key "your-api-key"

# Use environment variables
export MODELSLAB_API_KEY="your-api-key"
export MODELSLAB_TOKEN="your-bearer-token"
```

## Model Selection

Every generation command accepts a `--model` flag to specify which AI model to use. With 50,000+ models available, discovering the right one is a key workflow.

### Discovering Models

```bash
# Search by name
modelslab models search --search "flux"

# Filter by feature category
modelslab models search --feature imagen        # Image generation models
modelslab models search --feature video_fusion  # Video generation models
modelslab models search --feature audio_gen     # Audio/voice models
modelslab models search --feature llmaster      # LLM/chat models
modelslab models search --feature threed        # 3D generation models

# Filter by base model architecture
modelslab models search --base-model sdxl
modelslab models search --base-model flux

# Get detailed info about a specific model
modelslab models detail --id flux
modelslab models detail --id cogvideox
```

### Popular Models

| Model ID | Category | Description |
|----------|----------|-------------|
| `flux` | Image | Flux Dev — fast, high-quality images |
| `midjourney` | Image | MidJourney-style artistic images |
| `sdxl` | Image | Stable Diffusion XL base |
| `seedance-t2v` | Video | Seedance text-to-video |
| `seedance-i2v` | Video | Seedance image-to-video |
| `cogvideox` | Video | CogVideoX video generation |
| `eleven_multilingual_v2` | Audio | ElevenLabs multilingual TTS |
| `eleven_sound_effect` | Audio | ElevenLabs sound effects |
| `scribe_v1` | Audio | ElevenLabs speech-to-text |
| `meta-llama-3-8B-instruct` | LLM | Meta Llama 3 8B chat |
| `deepseek-ai-DeepSeek-R1-Distill-Llama-70B` | LLM | DeepSeek R1 reasoning |
| `gemini-2.0-flash-001` | LLM | Google Gemini 2.0 Flash |

### Setting a Default Model

```bash
# Set default model for image generation
modelslab config set generation.default_model flux

# Override per-command with --model
modelslab generate image --prompt "sunset" --model midjourney
```

Browse all models: https://modelslab.com/models

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
# Text to image (specify model with --model)
modelslab generate image --prompt "sunset over mountains" --model flux

# Image to image
modelslab generate image-to-image --prompt "oil painting style" --model midjourney --init-image https://...

# Text to video
modelslab generate video --prompt "ocean waves crashing" --model seedance-t2v

# Image to video
modelslab generate video --prompt "slowly zooming in" --model seedance-i2v --init-image https://...

# Text to speech
modelslab generate tts --text "Hello world" --language en

# Chat completion (OpenAI-compatible)
modelslab generate chat --message "Explain quantum computing" --model meta-llama-3-8B-instruct

# Music generation
modelslab generate music --prompt "upbeat electronic music"

# Check generation status
modelslab generate fetch --id 12345 --type image

# Skip polling (async)
modelslab generate image --prompt "sunset" --model flux --no-wait
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
