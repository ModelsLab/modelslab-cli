# AGENTS.md — ModelsLab CLI

> Guidelines for AI coding agents (Cursor, Copilot, Windsurf, Cline, etc.) working on this codebase.

## Project Overview

Go CLI for the ModelsLab AI platform. 103 commands across 14 groups covering auth, profile, keys, models, generate (image/video/audio/3D/chat), billing, wallet, subscriptions, teams, usage, config, MCP, docs, and shell completions. ~10k lines across 26 Go files.

**Stack**: Go 1.26 · Cobra + Viper · Charmbracelet (bubbletea, lipgloss, huh, glamour) · go-keyring · gojq · mcp-go · GoReleaser v2

## Build & Test

```bash
go build -o modelslab ./cmd/modelslab/    # Build binary
go test ./internal/... -v                  # Unit tests (no server needed)
go vet ./...                               # Lint
goreleaser release --snapshot --clean       # Full cross-platform build
```

Integration tests require a local Laravel server:
```bash
cd ~/Documents/GitHub/modelslab-frontend-v2 && php artisan serve --port=8888
MODELSLAB_TEST_TOKEN="<token>" MODELSLAB_TEST_API_KEY="<key>" go test ./tests/ -v -base-url http://127.0.0.1:8888
```

## File Map

| File | Purpose |
|------|---------|
| `cmd/modelslab/main.go` | Entry point, version ldflags |
| `internal/api/client.go` | HTTP client — retry, rate limiting, dual auth (`DoControlPlane`, `DoGeneration`) |
| `internal/auth/keyring.go` | OS keychain storage with JSON file fallback |
| `internal/cmd/root.go` | Root command, global flags (`--output`, `--jq`, `--profile`, `--base-url`, `--api-key`) |
| `internal/cmd/helpers.go` | `extractItems()`, `extractData()`, `firstNonNil()` — API response parsing |
| `internal/cmd/auth.go` | 12 auth commands |
| `internal/cmd/profile.go` | 6 profile commands |
| `internal/cmd/keys.go` | 5 API key commands |
| `internal/cmd/models.go` | 8 model discovery commands |
| `internal/cmd/generate.go` | 20 generation commands + async polling + file download |
| `internal/cmd/billing.go` | 10 billing commands + Stripe card tokenization |
| `internal/cmd/wallet.go` | 10 wallet commands |
| `internal/cmd/subscriptions.go` | 11 subscription commands |
| `internal/cmd/teams.go` | 7 team commands |
| `internal/cmd/usage.go` | 3 usage commands |
| `internal/cmd/config.go` | 6 config/profile commands |
| `internal/cmd/docs.go` | 2 docs commands |
| `internal/cmd/completion.go` | Shell completions (bash/zsh/fish/powershell) |
| `internal/cmd/mcp.go` | MCP serve + tools list |
| `internal/config/config.go` | Viper config (`~/.config/modelslab/config.toml`) |
| `internal/mcp/server.go` | MCP server with ~30 tools, stdio/SSE transports |
| `internal/output/formatter.go` | JSON, table, jq, key-value output formatting |
| `tests/integration_test.go` | 30 integration tests using subprocess execution |

## Patterns You Must Follow

### 1. Command Registration
Every command file uses `init()` to register commands on the parent group:
```go
func init() {
    parentCmd.AddCommand(myNewCmd)
    rootCmd.AddCommand(parentCmd) // only for top-level groups
}
```

### 2. Dual Auth — Pick the Right Method
- **Control plane** (`/api/agents/v1/*`): `client.DoControlPlane(method, path, body)` — uses Bearer token
- **Generation** (`/api/*`): `client.DoGeneration(method, path, body)` — uses API key in request body
- **Billing mutations**: `client.DoControlPlaneIdempotent(...)` — adds `Idempotency-Key` UUID header

Never mix these up. Check existing commands in the same group for which method to use.

### 3. API Response Parsing
Always use the helpers from `internal/cmd/helpers.go`:
```go
items := extractItems(result)   // for list endpoints — handles both {data:[...]} and {data:{items:[...]}}
data := extractData(result)     // for single-object endpoints — extracts data map
name := firstNonNil(m, "model_name", "name", "title")  // handles inconsistent field names
```

### 4. Output Handling
Every command must call `outputResult()` so `--output json` and `--jq` work:
```go
outputResult(cmd, result, func() {
    // human-readable output here (tables, key-value, etc.)
})
```

### 5. Error Handling with Exit Codes
The API client returns typed errors with semantic exit codes. Do not swallow errors — let them propagate:
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error |
| 3 | Auth error (401) |
| 4 | Rate limited (429) |
| 5 | Not found (404) |
| 6 | Payment error |
| 7 | Generation timeout |
| 10 | Network error |

### 6. Generation Commands
Generation commands use `pollAndDownload()` for async workflows:
1. POST to create generation → get `request_id`
2. Poll with exponential backoff (1s→2s→4s→8s→10s cap, 5min timeout)
3. Download output files to `./generated/`
4. `--no-wait` flag skips polling

### 7. Secrets
- API keys and tokens are stored in OS keychain, not config files
- `output.MaskSecret(s)` shows only last 4 chars — use this when displaying credentials
- Stripe publishable key is embedded in `billing.go` — this is intentional (it's a client-side key)
- Config key `api_key` is special-cased in `config.go` to store in keychain instead of config file

## Do Not

- **Do not add commands without registering them** in `init()` — they won't appear in the CLI
- **Do not use `data["token"]` for login** — the API returns `access_token` (both are checked as fallback)
- **Do not assume `data` is an array** — always use `extractItems()` which handles paginated responses
- **Do not hardcode API paths** — follow the pattern: control plane uses `/api/agents/v1/...`, generation uses `/api/v6/...`
- **Do not skip `go mod tidy`** after adding dependencies — CI will fail
- **Do not use GoReleaser v1 syntax** — this project uses v2 (e.g., `formats:` list not `format:` string)

## Adding a New Command (Step by Step)

1. Identify the command group (auth, billing, generate, etc.)
2. Open the corresponding `internal/cmd/<group>.go`
3. Define the command using `&cobra.Command{...}`
4. Add flags with `cmd.Flags().StringP(...)` etc.
5. In the `RunE` function:
   - Call `getClient()` to get the API client
   - Call the appropriate `Do*()` method
   - Call `outputResult()` for output
6. Register in `init()` on the parent command
7. Add a test in the appropriate test file

## Adding a New MCP Tool

1. Edit `internal/mcp/server.go`
2. Add a new `addTool()` call following existing patterns:
   ```go
   addTool("tool_name", "Description", map[string]interface{}{
       "type": "object",
       "properties": map[string]interface{}{
           "param": map[string]interface{}{"type": "string", "description": "..."},
       },
   }, func(args map[string]interface{}) (*mcp.CallToolResult, error) {
       // implementation
   })
   ```

## Design Spec

The authoritative spec for all 103 commands, API endpoints, and UX flows is in `cli.md` (1075 lines). Always consult it when adding features or fixing behavior.
