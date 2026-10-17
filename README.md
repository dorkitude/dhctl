# dhctl


A command-line interface for the DreamHost API, built with Go and Cobra.

## Features

- **Full API coverage**: every command DreamHost's API still offers (DNS records, announcement lists, and the API metacommands), plus raw access for anything new.
- **Authentication**: API key saved with `dhctl auth` (hidden prompt or stdin), `DREAMHOST_API_KEY` env-var override, and `dhctl whoami` to see what a key may call.
- **DNS**: list (filter by zone, type or name), add, remove, and `update`, which the API lacks (add the new record, then remove the old one).
- **Announcement lists**: lists, subscribers, subscribe/unsubscribe, and post or schedule announcements.
- **API keys**: `dhctl keys list` (`api-list_keys`).
- **Raw API**: `dhctl api <cmd> name=value ...` calls any command with your key.
- **Safe by default**: deletions confirm (or need `--yes`), posts never send without `--yes`, and the key never appears in output or errors.
- **Retries and accounts**: `--unique-id` makes a retried write safe; `--account` runs commands under another account you can access.
- **Output modes**: styled tables, or JSON with `--json`.

## Installation

### Homebrew (macOS/Linux)
```bash
brew install dorkitude/tap/dhctl
dhctl --help
```

This installs the release's prebuilt, signed and notarized binary (one universal binary on macOS), so it needs neither Go nor the Xcode Command Line Tools. If you installed dhctl from the tap when it was a formula built from source, switch once: `brew uninstall dhctl && brew install dorkitude/tap/dhctl`.

### apt (Ubuntu/Debian)
```bash
curl -fsSL https://dorkitude.github.io/dhctl-apt/key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/dhctl.gpg
echo "deb [signed-by=/usr/share/keyrings/dhctl.gpg] https://dorkitude.github.io/dhctl-apt stable main" | sudo tee /etc/apt/sources.list.d/dhctl.list
sudo apt update && sudo apt install dhctl
```
The signed repository is [dorkitude/dhctl-apt](https://github.com/dorkitude/dhctl-apt) (key fingerprint `E74D C111 92E3 ECE3 885A 27A6 A2C8 BCC9 D46C 6B47`).

### Scoop (Windows)
```powershell
scoop bucket add dorkitude https://github.com/dorkitude/scoop-bucket
scoop install dorkitude/dhctl
```

### Fedora/RHEL, or a single .deb
Download the package from the [releases page](https://github.com/dorkitude/dhctl/releases), then:
```bash
sudo dnf install ./dhctl_*_linux_amd64.rpm   # Fedora/RHEL (arm64 on ARM)
sudo apt install ./dhctl_*_linux_amd64.deb   # Ubuntu/Debian
```

### Install with Go
```bash
go install github.com/dorkitude/dhctl@latest
```

### From Source
```bash
git clone https://github.com/dorkitude/dhctl.git
cd dhctl
go build -o dhctl .
```

## Quick Start

> **IMPORTANT**  Agents like Claude Code, Codex, and Gemini should use the `--json` flag on all read operations.

No key yet? Every read command works with DreamHost's public demo key:
`DREAMHOST_API_KEY=6SHU5P2HLDAYECUM dhctl dns list`.

### 1. Authentication

Create a key in the DreamHost panel under **Billing & Account → Web Panel API**
(<https://panel.dreamhost.com/?tree=home.api>). Keys can only be made there. Tick only the
functions you need: `dns-*` for DNS, `announcement_list-*` for lists, and **All functions!**
only if you want `dhctl keys list`.

```bash
# Save your key (hidden prompt; or pipe it in: dhctl auth < keyfile)
dhctl auth

# Check that the key works
dhctl auth status

# Show the API commands this key may call
dhctl whoami

# Delete the saved key (revoke it in the panel too)
dhctl auth logout
```

### 2. DNS Records

```bash
# List every record on the account
dhctl dns list

# Filter by zone, type, or exact record name
dhctl dns list --zone example.com --type A
dhctl dns list --record www.example.com

# Show long values (TXT, DKIM) in full
dhctl dns list --zone example.com --wide

# Add a record (record is the full name; use example.com for the apex)
dhctl dns add www.example.com A 203.0.113.10 --comment "web"

# Change a value (adds the new record, then removes the old one)
dhctl dns update www.example.com A 203.0.113.10 203.0.113.20

# Remove a record (asks you to type the name; --yes skips)
dhctl dns rm old.example.com A 203.0.113.10
```

### 3. Announcement Lists

```bash
# List your announcement lists
dhctl announce list

# List a list's subscribers
dhctl announce subscribers news@example.com

# Subscribe and unsubscribe
dhctl announce subscribe news@example.com someone@example.org --name "Some One"
dhctl announce unsubscribe news@example.com someone@example.org

# Post now (refuses without --yes, since it emails every subscriber)
dhctl announce post news@example.com "October update" --file update.txt --type text --yes

# Schedule a post
echo "<p>Hi</p>" | dhctl announce post news@example.com "Hi" --type html --stamp "2026-11-01 09:00:00" --yes
```

### 4. API Keys and Raw Calls

```bash
# List the account's API keys (needs an All-functions key)
dhctl keys list

# Call any API command; prints the response's data as JSON
dhctl api dns-list_records
dhctl api <cmd> name=value ...
```

## CLI Guide

### Global flags

- `--json`: JSON output on every command
- `--account <number>`: run under another account you can access (from the panel's Manage Account page)
- `--unique-id <id>`: send this `unique_id` (up to 64 characters). DreamHost runs a given ID successfully only once per key, so reusing one makes a retried write safe.
- `--version`: print the version

### API coverage

DreamHost has removed its account, domain, mail, MySQL, user and rewards commands. The panel's
key page offers exactly these, and dhctl covers all of them:

| API command | dhctl |
| --- | --- |
| `api-list_accessible_cmds` | `dhctl whoami`, `dhctl auth status` |
| `api-list_keys` (All-functions keys only) | `dhctl keys list` |
| `dns-list_records` | `dhctl dns list` |
| `dns-add_record` | `dhctl dns add` (and `dns update`) |
| `dns-remove_record` | `dhctl dns rm` (and `dns update`) |
| `announcement_list-list_lists` | `dhctl announce list` |
| `announcement_list-list_subscribers` | `dhctl announce subscribers` |
| `announcement_list-add_subscriber` | `dhctl announce subscribe` |
| `announcement_list-remove_subscriber` | `dhctl announce unsubscribe` |
| `announcement_list-post_announcement` | `dhctl announce post` |

### DNS notes

- `rm` and `update` need an exact record, type and value match. dhctl looks the record up first, so a typo or a DreamHost-managed record gets a clear error instead of a bare API failure.
- `add` accepts A, AAAA, CNAME, NAPTR, NS, SRV and TXT, which is what the API takes. `list` also shows types DreamHost manages (MX, ALIAS, ...), usually with `editable=no`; change those in the panel.
- If `update`'s removal step fails, dhctl tells you both records now exist.
- DreamHost says changes can take a few hours to propagate.

### Announcement notes

- Lists are named by address, `listname@domain`.
- `post` reads the body from `--file` or stdin. Options: `--name` (sender), `--stamp "YYYY-MM-DD HH:MM:SS"`, `--charset`, `--type text|html`, `--duplicate-ok`.

### Troubleshooting

| Error | Meaning |
| --- | --- |
| `this_key_cannot_access_this_cmd` | The key lacks that function, or the command doesn't exist (DreamHost answers both the same way). Check with `dhctl whoami`. |
| `no record … (see dhctl dns list …)` | `rm`/`update` found no exact match. Copy the value from `dns list --wide`. |
| `… managed by DreamHost (editable=0)` | DreamHost owns that record. Change it in the panel. |
| `rate limited (slow_down_bucko …)` | Too many calls. Wait and retry; rejected calls don't count. |
| `no DreamHost API key` | Run `dhctl auth`, or set `DREAMHOST_API_KEY`. |

## Configuration and Credential Storage

By default, the key is stored under:

```text
~/.config/dhctl/
```

Files:

- `key` (API key, permissions 0600, directory 0700)

`DREAMHOST_API_KEY`, when set, overrides the saved key.

### Config directory override

```bash
DHCTL_CONFIG_DIR=/path/to/dir dhctl auth
```

## Current Scope and Gaps

What dhctl covers:

- Every command the DreamHost API offers today (see the coverage table)
- Safe DNS editing: look-before-delete, confirmations, and an update command

What the DreamHost API itself can't do (use the panel):

- Create or revoke API keys
- Change DreamHost-managed records (MX, ALIAS, hosting records with `editable=0`)
- Anything outside DNS and announcement lists (domains, mail, databases, users)

## Development

### Build
```bash
go build -o dhctl .
```

### Test
```bash
# Fake-API tests; no key needed
go test ./...

# Read-only check against the real API
DREAMHOST_API_KEY=6SHU5P2HLDAYECUM go run . dns list --zone groo.com
```

### Format
```bash
gofmt -w .
go vet ./...
```

### Contributing and releases

See [AGENTS.md](AGENTS.md).

## License

MIT
