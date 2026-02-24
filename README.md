# md365

AI-native CLI for Microsoft 365. Syncs calendars and contacts as local Markdown files.

## The Problem

If you run an AI agent that needs access to your Microsoft 365 data, you're stuck with the Graph API: OAuth token management, pagination, rate limits, and multi-second roundtrips for every lookup. That's fine for write operations, but for reads — "when's my next meeting?" or "what's Jane's email?" — it's way too much overhead.

## The Solution

md365 syncs your M365 calendars and contacts to local Markdown files with YAML frontmatter. Your agent reads local files. Writes still go through the API.

```bash
# Sync once
md365 sync

# Then search however you want
rg "jane doe" ~/.local/share/md365/
grep -r "team sync" ~/.local/share/md365/*/calendar/
cat ~/.local/share/md365/work/contacts/jane-doe.md
```

No tokens needed for reads. No API calls. Just files.

## How It Works

```
~/.local/share/md365/
├── work/
│   ├── calendar/
│   │   ├── 2026-02-24-team-sync.md
│   │   └── ...
│   └── contacts/
│       ├── jane-doe.md
│       └── ...
└── personal/
    ├── calendar/
    └── contacts/
```

### Calendar Event

```markdown
---
id: AAMkAGEx...
account: work
subject: Team Sync
start: 2026-02-24T16:00:00+01:00
end: 2026-02-24T18:00:00+01:00
location: https://zoom.us/j/123456
organizer: colleague@company.com
attendees:
  - colleague@company.com
  - you@company.com
response: accepted
online_meeting: true
last_modified: 2026-02-18T10:30:00Z
---

# Team Sync

Weekly team synchronization meeting.
```

### Contact

```markdown
---
id: AAMkAGE4...
account: personal
display_name: Jane Doe
emails:
  - jane@example.com
phones:
  - "+49 123 456 789"
company: Acme Corp
job_title: Engineer
---

# Jane Doe

📧 jane@example.com
📱 +49 123 456 789
🏢 Acme Corp — Engineer
```

## Usage

```bash
md365 sync                              # Sync all accounts
md365 sync --account work               # Sync one account

md365 cal list                           # Upcoming events (14 days)
md365 cal list --from 2026-02-24 --to 2026-02-28
md365 cal list --search sync

md365 cal create --account work \        # Create event via API
  --subject "Lunch" \
  --start "2026-03-01T12:00" \
  --end "2026-03-01T13:00"

md365 cal delete --account work --id <event-id>

md365 contacts search doe               # Search local contacts

md365 mail send --account work \         # Send mail via API
  --to "colleague@company.com" \
  --subject "Hello" --body "Text"

md365 auth login --account work          # Device code OAuth login
md365 auth status                        # Token status
```

## Cross-Tenant Guard

If you manage multiple accounts, md365 prevents you from accidentally sending mail or creating events from the wrong one. Configure associated domains per account:

```yaml
accounts:
  work:
    domains:
      - company.com
  personal:
    domains:
      - gmail.com
```

Sending from `personal` to `colleague@company.com` will be blocked with a suggestion to use `--account work`. Override with `--force`.

## Setup

### 1. Add an Account

Interactive setup (guided TUI):
```bash
md365 auth add -i
```

Or non-interactive (AI-friendly):
```bash
md365 auth add --name work --hint you@company.com \
  --scopes "Calendars.ReadWrite,Contacts.ReadWrite,User.Read,Mail.Send" \
  --domains "company.com" --login
```

md365 ships with a built-in app registration — no Azure setup needed. If your tenant requires a custom app, you can set `client_id` per account in the config.

### 2. Login and Sync

```bash
md365 auth login --account work
md365 sync
```

### Auth Flows

Most tenants work with the default **Device Code Flow**. If your tenant blocks it (Conditional Access), use **Authorization Code Flow with PKCE**:

```yaml
accounts:
  work:
    auth_flow: authcode    # opens browser instead of device code
    hint: you@company.com
    scope: "offline_access Calendars.ReadWrite User.Read"
```

### Configuration

Config lives at `~/.config/md365/config.yaml`:

```yaml
accounts:
  work:
    hint: "you@company.com"
    scope: "offline_access Calendars.ReadWrite Contacts.ReadWrite User.Read Mail.Send"
    domains:
      - company.com
  personal:
    hint: "you@outlook.com"
    scope: "offline_access Calendars.ReadWrite Contacts.ReadWrite User.Read"
    domains:
      - gmail.com
```

## Token Storage

Tokens are stored exclusively in the system keyring (gnome-keyring, macOS Keychain, Windows Credential Manager). A running keyring daemon is required — no file fallback.

The `offline_access` scope enables refresh tokens, so you only need to log in once per account. Tokens refresh automatically on use and remain valid for up to 90 days of inactivity.

## Installation

### Homebrew (macOS & Linux)

```bash
brew install lcorneliussen/md365/md365
```

### AUR (Arch Linux)

```bash
yay -S md365-bin
```

### Go Install

```bash
go install github.com/lcorneliussen/md365@latest
```

### GitHub Releases

Download pre-built binaries for Linux, macOS, and Windows from [Releases](https://github.com/lcorneliussen/md365/releases).

### Build from Source

```bash
git clone https://github.com/lcorneliussen/md365.git
cd md365
go build -o md365 .
```

## Sync Details

- **Events:** Full window sync (past 30 → future 90 days). Remotely deleted events are removed locally.
- **Contacts:** Delta sync via Graph API for incremental updates.
- **Direction:** One-way (remote → local). Local files are a read-only cache.

## License

MIT
