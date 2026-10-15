# dhctl

A CLI for the [DreamHost API](https://help.dreamhost.com/hc/en-us/articles/217560167-API-overview),
covering every command it still has: DNS records, announcement lists and the API
metacommands, plus raw access for anything new.

## Install

### Homebrew (macOS/Linux)

```bash
brew install dorkitude/tap/dhctl
```

### apt (Ubuntu/Debian)

```bash
curl -fsSL https://dorkitude.github.io/dhctl-apt/key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/dhctl.gpg
echo "deb [signed-by=/usr/share/keyrings/dhctl.gpg] https://dorkitude.github.io/dhctl-apt stable main" | sudo tee /etc/apt/sources.list.d/dhctl.list
sudo apt update && sudo apt install dhctl
```

The repository is [dorkitude/dhctl-apt](https://github.com/dorkitude/dhctl-apt). Its signing key
fingerprint is `E74D C111 92E3 ECE3 885A 27A6 A2C8 BCC9 D46C 6B47`.

### Scoop (Windows)

```powershell
scoop bucket add dorkitude https://github.com/dorkitude/scoop-bucket
scoop install dorkitude/dhctl
```

### Fedora/RHEL (.rpm), or a .deb without the repository

Download the package for your architecture from the
[releases page](https://github.com/dorkitude/dhctl/releases), then:

```bash
sudo dnf install ./dhctl_*_linux_amd64.rpm   # Fedora/RHEL (arm64 on ARM)
sudo apt install ./dhctl_*_linux_amd64.deb   # Ubuntu/Debian
```

### Go

```bash
go install github.com/dorkitude/dhctl@latest
```

Release archives for macOS, Linux and Windows (amd64, arm64) are on the releases page too.

## Auth

Create an API key in the DreamHost panel (**Account → API Keys**,
<https://panel.dreamhost.com/?tree=home.api>). Grant it only what you need; for DNS
that's `dns-list_records`, `dns-add_record` and `dns-remove_record`. Then:

```bash
dhctl auth < keyfile      # or just `dhctl auth` for a hidden prompt
dhctl auth status         # checks the key works
dhctl whoami              # lists the commands the key may call
```

The key is saved to `~/.config/dhctl/key` (0600; override the directory with
`$DHCTL_CONFIG_DIR`). `$DREAMHOST_API_KEY` overrides the saved key. dhctl never prints
the key, including in errors.

## What the API covers

DreamHost removed its account, domain, mail, MySQL, user and rewards commands. The panel's
key page (2026-10) offers exactly these, and dhctl covers all of them:

| API command | dhctl |
| --- | --- |
| `api-list_accessible_cmds` | `dhctl whoami`, `dhctl auth status` |
| `api-list_keys` (all-functions keys only) | `dhctl keys list` |
| `dns-list_records` | `dhctl dns list` |
| `dns-add_record` | `dhctl dns add` (and `dns update`) |
| `dns-remove_record` | `dhctl dns rm` (and `dns update`) |
| `announcement_list-list_lists` | `dhctl announce list` |
| `announcement_list-list_subscribers` | `dhctl announce subscribers` |
| `announcement_list-add_subscriber` | `dhctl announce subscribe` |
| `announcement_list-remove_subscriber` | `dhctl announce unsubscribe` |
| `announcement_list-post_announcement` | `dhctl announce post` |

Global flags: `--json` on every command, `--account <number>` to run under another
account you can access, and `--unique-id <id>` (up to 64 characters). DreamHost runs a
given `unique_id` successfully only once per key, so reusing one makes a retried write
safe. A rate-limited call (`slow_down_bucko`) gets a clear "wait and retry" error.

Try it read-only with DreamHost's public demo key: `DREAMHOST_API_KEY=6SHU5P2HLDAYECUM dhctl dns list`.

## DNS

```bash
dhctl dns list                                   # every record on the account
dhctl dns list --zone example.com --type A       # filter by zone, type or --record
dhctl dns add www.example.com A 203.0.113.10 --comment "web"
dhctl dns update www.example.com A 203.0.113.10 203.0.113.20   # add new, then remove old
dhctl dns rm old.example.com A 203.0.113.10      # asks you to type the name; --yes skips
```

- `record` is the full name (`www.example.com`, or `example.com` for the apex).
- `rm` needs an exact record, type and value match. It looks the record up first and
  explains a typo or a DreamHost-managed (`editable=0`) record instead of failing blind.
  Off a terminal it refuses without `--yes`.
- The API has **no update**. `dns update` adds the new record, then removes the old one;
  if the removal fails, it tells you both exist.
- `add` accepts A, AAAA, CNAME, NAPTR, NS, SRV and TXT (what the API takes). Long values
  are truncated in the table; `--wide` shows them in full.
- New records can take a few hours to propagate (DreamHost's note).
- Every command takes `--json`.

## Announcement lists

Lists are named by address, `listname@domain`.

```bash
dhctl announce list
dhctl announce subscribers news@example.com
dhctl announce subscribe news@example.com someone@example.org --name "Some One"
dhctl announce unsubscribe news@example.com someone@example.org      # confirms; --yes skips
dhctl announce post news@example.com "October update" --file update.txt --type text --yes
```

`post` emails every subscriber, so it refuses without `--yes`. The body comes from `--file`
or stdin. Optional: `--name` (sender), `--stamp "YYYY-MM-DD HH:MM:SS"` (schedule),
`--charset`, `--type text|html`, `--duplicate-ok`.

## Anything else

```bash
dhctl api domain-list_domains
dhctl api <cmd> name=value ...
```

`dhctl api` adds `key`, `format=json` and a `unique_id`, and prints the response's
`data` field as JSON.

## Development

```bash
go test ./...
go build -o dhctl .
```

Releases: push a `vX.Y.Z` tag. GitHub Actions runs the tests and GoReleaser (archives,
.deb/.rpm), then `scripts/publish-apt.sh` signs the .debs into
[dhctl-apt](https://github.com/dorkitude/dhctl-apt) (secrets `APT_GPG_PRIVATE_KEY`,
`APT_DEPLOY_KEY`). Afterwards, run `scripts/publish-packages.sh vX.Y.Z` (needs `gh` with push
access) to update the Homebrew tap and the Scoop bucket.

MIT licensed.
