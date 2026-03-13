# Release plan (v0.x)

## v0.1.0 — Usable baseline (target: next release)

Goal: stable single-binary CLI for local multi-agent log triage.

Scope:
- feed/stats/agent commands (table + JSON output)
- multi-file merge, time-window filters, agent filter
- parser hardening and deterministic ordering
- CI checks (fmt/test/staticcheck)
- first tagged release with checksums

Exit criteria:
- `go test ./...` green on CI
- install + core commands documented and reproducible
- release artifact published from tag

## v0.2.0 — Reliability + operator UX

Goal: better behavior on real, messy logs.

Scope:
- explicit tail/recent mode UX for feed
- improved parse diagnostics and skip counters
- optional support for compressed logs (`.gz`)
- stronger JSON schema/field mapping controls

Exit criteria:
- predictable behavior on malformed/mixed records
- clearer operator feedback without noisy output

## v0.3.0 — Automation and integrations

Goal: easier embedding in scripts and pipelines.

Scope:
- machine-readable summaries for all commands
- structured error codes
- optional GitHub Actions/CI summary formatter
- package/distribution polish (Homebrew/Scoop candidate)

Exit criteria:
- stable output contracts for automation use
- documented integration examples
