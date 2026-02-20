# CLAUDE.md — ModelsLab CLI

## What is this project?

A Go CLI for the [ModelsLab](https://modelslab.com) platform. It covers account management, model discovery, AI generation (image/video/audio/3D/chat), billing, wallet, subscriptions, teams, usage analytics, and MCP server mode. ~10k lines of Go across 26 files, 103 subcommands in 14 groups.

## Quick Reference

```bash
# Build
go build -o modelslab ./cmd/modelslab/

# Run unit tests
go test ./internal/... -v

# Run integration tests (requires local server)
cd ~/Documents/GitHub/modelslab-frontend-v2 && php artisan serve --port=8888 &
MODELSLAB_TEST_TOKEN="<token>" MODELSLAB_TEST_API_KEY="<key>" go test ./tests/ -v -base-url http://127.0.0.1:8888

# Snapshot release build (no publish)
goreleaser release --snapshot --clean

# Lint
go vet ./...
```

## Project Structure

```
cmd/modelslab/main.go          # Entry point, version injection via ldflags
internal/
  api/client.go                # HTTP client (retry, rate limiting, dual auth)
  api/client_test.go           # 14 unit tests with httptest
  auth/keyring.go              # Credential storage (OS keychain + file fallback)
  cmd/
    root.go                    # Root command, global flags, getClient(), outputResult()
    helpers.go                 # extractItems(), extractData(), firstNonNil()
    auth.go                    # 12 auth commands (login, signup, logout, tokens, etc.)
    profile.go                 # 6 profile commands
    keys.go                    # 5 API key commands
    models.go                  # 8 model discovery commands
    generate.go                # 20 generation commands + polling + download
    billing.go                 # 10 billing commands + Stripe tokenization
    wallet.go                  # 10 wallet commands
    subscriptions.go           # 11 subscription commands
    teams.go                   # 7 team commands
    usage.go                   # 3 usage analytics commands
    config.go                  # 6 config/profile commands
    docs.go                    # 2 docs commands
    completion.go              # Shell completions
    mcp.go                     # MCP serve + tools list commands
  config/config.go             # Viper config management
  config/config_test.go        # 7 unit tests
  mcp/server.go                # MCP server (~30 tools, stdio/SSE)
  output/formatter.go          # JSON, table, jq, key-value formatting
  output/formatter_test.go     # 8 unit tests
tests/integration_test.go      # 30 integration tests against local server
```

## Architecture

### Dual Authentication

The CLI uses two credential types stored in the OS keychain (go-keyring) with file fallback at `~/.config/modelslab/profiles/{profile}.json`:

- **Bearer token** — for control plane endpoints (`/api/agents/v1/*`) — profile, keys, billing, wallet, subscriptions, teams, usage
- **API key** — for generation endpoints (`/api/*`) — image, video, audio, 3D, chat

`internal/api/client.go` exposes:
- `DoControlPlane(method, path, body)` — uses Bearer token
- `DoGeneration(method, path, body)` — uses API key in JSON body
- `DoControlPlaneIdempotent(...)` — adds `Idempotency-Key` UUID header for billing mutations

### API Response Patterns

The backend returns two patterns. The `extractItems()` helper in `helpers.go` handles both:

```
Paginated:  { "data": { "items": [...], "pagination": {...} }, "error": null }
Direct:     { "data": [...], "error": null }
Single:     { "data": { "field": "value" }, "error": null }
```

Use `extractItems()` for lists, `extractData()` for single objects.

### Field Name Ambiguity

API responses use inconsistent field names. The `firstNonNil(map, keys...)` helper tries multiple field names:
```go
name := firstNonNil(item, "model_name", "name", "title")
```

### Generation Polling

Generation commands use async polling (`pollAndDownload` in `generate.go`):
1. Submit request → get `request_id`
2. Poll `GET /api/v6/images/fetch/{id}` with exponential backoff (1s → 2s → 4s → 8s → 10s cap)
3. Timeout after 5 minutes
4. Auto-download output files to `./generated/` directory

`--no-wait` flag skips polling and returns immediately.

### Output Modes

Every command supports `--output json` and `--jq '<expression>'` via `outputResult()` in `root.go`. Human mode prints colored tables. The `internal/output/formatter.go` handles all formatting.

### Config Precedence

CLI flags → env vars (`MODELSLAB_*`) → project config → user config (`~/.config/modelslab/config.toml`) → defaults.

### MCP Server

`internal/mcp/server.go` registers ~30 tools covering both control plane and generation endpoints. Supports `stdio` (default) and `sse` transports. Used by Claude Desktop/Code:
```json
{ "mcpServers": { "modelslab": { "command": "modelslab", "args": ["mcp", "serve"] } } }
```

## Key Conventions

- **Command registration**: Each `internal/cmd/*.go` file creates an `init()` function that adds commands to the root or parent group via `rootCmd.AddCommand()`.
- **Error handling**: `api.Client` returns semantic exit codes (3=auth, 4=rate-limit, 5=not-found, 6=payment, 7=timeout, 10=network). Commands propagate these via `os.Exit()`.
- **Retry logic**: The HTTP client auto-retries 429 and 5xx responses with exponential backoff (max 3 retries). Rate limit headers `X-RateLimit-Remaining` and `X-RateLimit-Reset` are respected.
- **Secrets masking**: `output.MaskSecret()` shows only last 4 chars. Used when displaying tokens/keys.
- **Stripe tokenization**: `billing.go` embeds the Stripe publishable key and tokenizes card data client-side via `DoStripe()` before sending the token to the backend.

## Testing

### Unit Tests (no server needed)
```bash
go test ./internal/api/ -v      # HTTP client tests with httptest mock server
go test ./internal/config/ -v   # Config management tests
go test ./internal/output/ -v   # Output formatting tests
```

### Integration Tests (need local Laravel server)
```bash
# Start backend
cd ~/Documents/GitHub/modelslab-frontend-v2 && php artisan serve --port=8888

# Run with credentials
MODELSLAB_TEST_TOKEN="<token>" MODELSLAB_TEST_API_KEY="<key>" go test ./tests/ -v -base-url http://127.0.0.1:8888
```

Test user: `test@example.com` / `password` (from `DatabaseSeeder.php`).

Integration tests use `runCLI()` helper that executes the compiled binary as a subprocess. Authenticated tests are gated behind env vars — they skip if credentials are not set.

## Build & Release

- **GoReleaser v2** — `.goreleaser.yml` builds for darwin/linux/windows × amd64/arm64
- **CI** — `.github/workflows/ci.yml` runs tests on push/PR to main
- **Release** — `.github/workflows/release.yml` triggered by `v*` tags, runs GoReleaser
- **Install script** — `install.sh` detects platform and downloads from GitHub Releases
- **Package managers** — Homebrew tap (`modelslab/tap`), Scoop bucket (`ModelsLab/scoop-bucket`), deb/rpm via nfpm

## Common Tasks for Agents

### Adding a new command
1. Create or edit the appropriate file in `internal/cmd/`
2. Register the command in `init()` by adding it to its parent command
3. Use `getClient()` to get the API client
4. Use `DoControlPlane()` or `DoGeneration()` for the HTTP call
5. Use `outputResult()` to handle JSON/human output
6. Use `extractItems()` or `extractData()` to parse the response

### Adding a new MCP tool
1. Edit `internal/mcp/server.go`
2. Add a new `addTool()` call with the tool name, description, JSON schema, and handler function

### Modifying API response handling
Check `internal/cmd/helpers.go` for the shared response parsing helpers. If the API returns a new field name variant, add it to the `firstNonNil()` call for that command.

## Design Document

The full design spec is in `cli.md` (1075 lines). It contains the complete API endpoint mapping, all 103 commands, UX flows, and architectural decisions. Consult it for the authoritative specification.
