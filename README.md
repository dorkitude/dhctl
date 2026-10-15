# dhctl

A small CLI for the [DreamHost API](https://help.dreamhost.com/hc/en-us/articles/217560167-API-overview):
list, add and remove DNS records, plus raw access to any other API command.

```bash
go install github.com/dorkitude/dhctl@latest
```

Prebuilt binaries for macOS and Linux (amd64, arm64) are on the
[releases page](https://github.com/dorkitude/dhctl/releases).

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

## DNS

```bash
dhctl dns list                                   # every record on the account
dhctl dns list --zone example.com --type A       # filter by zone, type or --record
dhctl dns add www.example.com A 203.0.113.10 --comment "web"
dhctl dns rm old.example.com A 203.0.113.10      # asks you to type the name; --yes skips
```

- `record` is the full name (`www.example.com`, or `example.com` for the apex).
- `rm` needs an exact record, type and value match. It looks the record up first and
  explains a typo or a DreamHost-managed (`editable=0`) record instead of failing blind.
  Off a terminal it refuses without `--yes`.
- The API has **no update**. To change a record, `add` the new one, then `rm` the old one.
- New records can take a few hours to propagate (DreamHost's note).
- Every command takes `--json`.

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

Releases: push a `vX.Y.Z` tag; GitHub Actions runs the tests and GoReleaser.

MIT licensed.
