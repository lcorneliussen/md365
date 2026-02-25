# AGENTS.md — md365

## Project
- **What:** AI-native CLI for Microsoft 365 — calendars, contacts, and mail as Markdown
- **Language:** Go
- **Repo:** github.com/lcorneliussen/md365
- **Binary:** `md365`

## Build & Test
```bash
go build ./...                          # compile check
go build -o ~/.local/bin/md365 .        # install locally
```

No test suite yet — test manually:
```bash
md365 auth refresh --account private
md365 sync --account private
md365 cal list --account private --from 2026-01-01 --to 2026-12-31
```

## Config
- Config: `~/.config/md365/config.yaml`
- Tokens: System keyring (secret-tool, service=md365) — NOT file-based
- Data: `~/.local/share/md365/<account>/calendar|contacts/*.md`

### Accounts
Account names (not emails!) are used for all commands:
| Account | Email |
|---|---|
| `private` | lars@corneliussen.de |
| `dcg` | lars.corneliussen@dcg-waltrop.de |
| `talendos` | lc@talendos.com |
| `oms` | lars.corneliussen@itc-service.eu |

## Release Flow
GoReleaser handles everything. **Do NOT use `gh release create`.**

1. Commit with conventional commit messages (`fix:`, `feat:`, etc.)
2. Tag: `git tag v0.x.x`
3. Push: `git push && git push origin v0.x.x`
4. GoReleaser (GitHub Actions) automatically:
   - Builds binaries (linux/darwin/windows × amd64/arm64)
   - Creates GitHub Release with changelog from commits
   - Updates Homebrew tap (lcorneliussen/homebrew-md365)
5. If notes need editing after: `gh release edit v0.x.x --notes "..."`

## Architecture
```
main.go
cmd/           # Cobra commands (root, auth, cal, contacts, mail, sync)
internal/
  auth/        # OAuth2 device code flow, token refresh, keyring storage
  cal/         # Calendar list, create, delete
  config/      # Config loading, cross-tenant checks
  contacts/    # Contact sync
  graph/       # Microsoft Graph API client, types
  mail/        # Mail send
  sync/        # Sync engine, markdown file writer
```

## Key Decisions
- **Timezone:** Uses `cfg.Timezone` (from config.yaml, e.g. "Europe/Berlin") everywhere. Never hardcode UTC.
- **Graph API Event struct:** Optional fields use pointer types + `omitempty` to avoid sending empty structs (causes HTTP 400).
- **Frontmatter dates:** RFC3339 with local timezone offset (e.g. `+01:00` CET, `+02:00` CEST).
