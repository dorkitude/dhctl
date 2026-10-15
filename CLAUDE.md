# CLAUDE.md

dhctl is a Go CLI for the DreamHost API (Cobra + lipgloss), modeled on
[simple](https://github.com/dorkitude/simple).

- `main.go`: entry point. `cmd/`: Cobra commands (`auth`, `whoami`, `dns list|add|rm`, `api`).
- `internal/client/`: the API client. Every call is a GET to `https://api.dreamhost.com/`
  with `key`, `cmd`, `format=json` and `unique_id`. Responses are
  `{"result":"success|error","data":...}`. **Never let the key reach output or errors**
  (`redact` covers transport errors and odd bodies; keep tests for it).
- `internal/config/`: the key lives in `~/.config/dhctl/key` (0600), or `$DREAMHOST_API_KEY`.
- Tests run against `httptest` fakes via the hidden `--base-url` flag: `go test ./...`.
- Release: tag `vX.Y.Z` and push. `.github/workflows/release.yml` runs tests + GoReleaser.
  Install with `go install github.com/dorkitude/dhctl@latest`.
- Destructive commands confirm on a TTY and require `--yes` otherwise.
