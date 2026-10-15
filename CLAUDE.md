# CLAUDE.md

dhctl is a Go CLI for the DreamHost API (Cobra + lipgloss), modeled on
[simple](https://github.com/dorkitude/simple).

- `main.go`: entry point. `cmd/`: Cobra commands (`auth`, `whoami`, `keys`, `dns list|add|update|rm`,
  `announce ...`, `api`). The README's "What the API covers" table maps every DreamHost
  command to a dhctl command; keep it complete when DreamHost adds commands (the panel's
  API-key page lists them all).
- `internal/client/`: the API client. Every call is a GET to `https://api.dreamhost.com/`
  with `key`, `cmd`, `format=json` and `unique_id`. Responses are
  `{"result":"success|error","data":...}`. **Never let the key reach output or errors**
  (`redact` covers transport errors and odd bodies; keep tests for it).
- `internal/config/`: the key lives in `~/.config/dhctl/key` (0600), or `$DREAMHOST_API_KEY`.
- Tests run against `httptest` fakes via the hidden `--base-url` flag: `go test ./...`.
- Release: tag `vX.Y.Z` and push. `.github/workflows/release.yml` runs tests, GoReleaser
  and `scripts/publish-apt.sh` (signed apt repo dorkitude/dhctl-apt on GitHub Pages). Then
  run `scripts/publish-packages.sh vX.Y.Z` locally for the Homebrew tap
  (dorkitude/homebrew-tap) and Scoop bucket (dorkitude/scoop-bucket). Same pattern as cfctl
  and skimmer.
- Destructive commands confirm on a TTY and require `--yes` otherwise.
- Read-only live check: `DREAMHOST_API_KEY=6SHU5P2HLDAYECUM` is DreamHost's public demo key.
