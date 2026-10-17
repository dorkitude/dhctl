# AGENTS.md

## Purpose
Ship safe, small, tested changes to `dhctl` with docs that match behavior.

## Operating Rules
- Keep patches minimal and focused; avoid drive-by refactors.
- Prefer `go test`/existing scripts; do not invent parallel tooling.
- Never rewrite published history (`reset --hard`, force-push) unless explicitly requested.
- If behavior changes, update `README.md` in the same change.
- **Never let the API key reach output, errors, logs, or the repo.** The key travels in the
  request URL, so `internal/client` redacts it from transport errors and odd response bodies;
  keep the tests for that. The only key allowed in the repo is DreamHost's public demo key
  `6SHU5P2HLDAYECUM` (allowlisted in `.gitleaks.toml`).
- Destructive commands confirm on a TTY and require `--yes` otherwise. `announce post`
  always requires `--yes`.

## Layout
- `main.go`: entry point.
- `cmd/`: Cobra commands: `auth`, `whoami`, `keys`, `dns list|add|update|rm`, `announce ...`, `api`.
- `internal/client/`: the API client. Every call is a GET to `https://api.dreamhost.com/` with
  `key`, `cmd`, `format=json`, `unique_id` (and `account` when set). Responses are
  `{"result":"success|error","data":...}`.
- `internal/config/`: the key in `~/.config/dhctl/key` (0600) or `$DREAMHOST_API_KEY`.
- `internal/ui/`: lipgloss styles.

## Quality Bar
- Run `gofmt -l .`, `go vet ./...` and `go test ./...` before finalizing code changes.
- For command or flag changes, add or update tests in `cmd/cmd_test.go`. Tests run against an
  `httptest` fake through the hidden `--base-url` flag.
- For client changes, add or update `internal/client/client_test.go`.
- Run `gitleaks detect --no-git --source .` before every push, and **stop if it reports
  anything**. Don't pipe it into `tail`, which hides the exit code.
- If tests are skipped, state exactly what was skipped and why.

## Docs Freshness
- The README's "API coverage" table must list every command DreamHost offers. The
  authoritative list is the checkbox list on the panel's Web Panel API page
  (`?tree=home.api`, checkboxes `func_<cmd>`). The help-center docs lag behind it (they omit
  announcement lists).
- DreamHost answers `this_key_cannot_access_this_cmd` both for commands a key lacks and for
  commands that don't exist, so you can't probe for commands with a narrow key.
- Keep README examples runnable with the current CLI.

## Fast Checks
```bash
go test ./...
DREAMHOST_API_KEY=6SHU5P2HLDAYECUM go run . whoami        # public, read-only demo key
DREAMHOST_API_KEY=6SHU5P2HLDAYECUM go run . announce list
goreleaser release --snapshot --clean --skip=publish     # packaging check (rm -rf dist after)
```

## Release + Package Managers (Owners Only)
- Releases are cut from `main` with semver tags `vX.Y.Z` (patch numbers can be large, e.g. `v0.3.100`).
- Channels:
  - **GitHub release**: macOS/Linux tarballs (plus a universal `darwin_all` macOS binary, signed and
    notarized), Windows zips, `.deb`/`.rpm` (GoReleaser, `.goreleaser.yaml`).
  - **apt**: signed repo [dorkitude/dhctl-apt](https://github.com/dorkitude/dhctl-apt) on GitHub
    Pages, updated by the release workflow (`scripts/publish-apt.sh`).
  - **Homebrew**: `Casks/dhctl.rb` in [dorkitude/homebrew-tap](https://github.com/dorkitude/homebrew-tap):
    the prebuilt binary, so installs need neither Go nor the Xcode Command Line Tools.
  - **Scoop**: `dhctl.json` in [dorkitude/scoop-bucket](https://github.com/dorkitude/scoop-bucket).
  - **Go**: `go install github.com/dorkitude/dhctl@vX.Y.Z` (version comes from build info).
- Required repo secrets on `dorkitude/dhctl` (already set; they exist nowhere else):
  - `APT_GPG_PRIVATE_KEY`: signs the apt repo (fingerprint `E74D C111 92E3 ECE3 885A 27A6 A2C8 BCC9 D46C 6B47`).
  - `APT_DEPLOY_KEY`: SSH deploy key with write access to `dorkitude/dhctl-apt` only.
  - `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`: the Developer ID Application certificate (base64 .p12) and
    its password; `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_ISSUER_ID`: an App Store
    Connect API key (base64 .p8) for notarization. GoReleaser signs and notarizes the macOS binaries.
- Flow:
  1. `git push origin main`
  2. `git tag -a vX.Y.Z -m "vX.Y.Z: <summary>" && git push origin vX.Y.Z`
  3. Wait for the `release` workflow: tests, GoReleaser, then the apt publish step. Check with
     `gh run list -R dorkitude/dhctl` and `gh release view vX.Y.Z -R dorkitude/dhctl`
     (expect 12 assets: checksums, 5 tarballs including darwin_all, 2 zips, 2 .deb, 2 .rpm).
  4. `scripts/publish-packages.sh vX.Y.Z` (needs `gh` with push access to the tap and bucket;
     `DRY_RUN=1` prints instead of pushing).
  5. Verify every channel installs the new version:
     - apt, in a clean container:
       `docker run --rm ubuntu:24.04 bash -c '<README apt steps> && dhctl --version'`
     - Homebrew: `brew update && brew install dorkitude/tap/dhctl && dhctl --version` (or `brew upgrade`)
     - Scoop: the manifest's hashes match the release zips
       (`gh api repos/dorkitude/scoop-bucket/contents/dhctl.json`).
- A release isn't done until all channels show the new version.
