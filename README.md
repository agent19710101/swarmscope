# swarmscope

`swarmscope` is a Go CLI for **watching and summarizing multi-agent run logs** from mixed JSON/JSONL sources (including `.jsonl.gz`).

It helps answer:
- What did each agent do, in order?
- Which tools/actions dominate a run?
- Where did failures happen?

## Why

Agent orchestration is hot, but logs are fragmented and noisy. `swarmscope` gives you one terminal-first feed and summary for fast debugging, incident triage, and demos.

## Install

```bash
go install github.com/agent19710101/swarmscope/cmd/swarmscope@latest
```

Or build locally:

```bash
go build ./cmd/swarmscope
```

## Usage

```bash
# unified chronological feed
swarmscope feed --input ./examples/run.jsonl

# latest 20 events (tail mode)
swarmscope feed --input ./examples/run.jsonl --limit 20 --tail

# merge multiple log files (comma-separated)
swarmscope feed --input ./examples/run.jsonl,./examples/run-extra.jsonl

# read compressed JSONL directly
swarmscope feed --input ./logs/run.jsonl.gz

# focus only on a time window
swarmscope feed --input ./examples/run.jsonl \
  --since 2026-03-13T01:10:03Z --until 2026-03-13T01:10:09Z

# or use a relative window from "now"
swarmscope stats --input ./examples/run.jsonl --last 30m

# drill down to one (or more) agents
swarmscope feed --input ./examples/run.jsonl --agent coder-a
swarmscope stats --input ./examples/run.jsonl --agent coder-a,reviewer

# focus on one input source when merging multiple files
swarmscope feed --input ./logs/a.jsonl,./logs/b.jsonl --source b.jsonl
swarmscope stats --input ./logs/a.jsonl,./logs/b.jsonl --source ./logs/a.jsonl

# focus events by message text (case-insensitive)
swarmscope feed --input ./examples/run.jsonl --contains "edge-case"
swarmscope stats --input ./examples/run.jsonl --contains "failed"

# summary table by agent + action + status
swarmscope stats --input ./examples/run.jsonl

# per-agent activity summary (events, first/last seen, unique actions/statuses)
swarmscope agent --input ./examples/run.jsonl

# machine-readable output for piping/automation
swarmscope feed --input ./examples/run.jsonl --format json
swarmscope stats --input ./examples/run.jsonl --format json
swarmscope agent --input ./examples/run.jsonl --format json

# read a single payload from stdin
cat ./examples/run.jsonl | swarmscope stats --input -

# custom field mapping for non-standard logs
swarmscope feed --input ./examples/run-custom.jsonl --map ./examples/map-profile.json

# strict mode (fail fast when required canonical fields are missing)
swarmscope stats --input ./examples/run-custom.jsonl --map ./examples/map-profile.json --strict
```

## Demo output

```text
001 01:10:00  planner       plan          ok      decomposed issue #42
002 01:10:03  coder-a       edit          ok      updated parser
003 01:10:06  coder-b       test          fail    go test ./... failed
004 01:10:09  reviewer      review        ok      requested edge-case fix
```

```text
events:   4
window:   2026-03-13T01:10:00Z -> 2026-03-13T01:10:09Z (9s)

agents:
  planner               1
  coder-a               1
  coder-b               1
  reviewer              1
```

## Event format auto-detection

`swarmscope` normalizes common fields from each record:

- timestamp: `ts`, `time`, `timestamp`, `created_at` (RFC3339 or Unix epoch seconds/milliseconds/microseconds/nanoseconds)
- agent: `agent`, `agent_name`, `worker`, `session`
- action: `action`, `event`, `type`, `tool`
- status: `status`, `level`, `result`
- message: `message`, `msg`, `summary`, `content`

You can extend or replace these aliases with `--map <profile.json>`.

Example mapping profile:

```json
{
  "timestamp": ["when"],
  "agent": ["actor"],
  "action": ["op"],
  "status": ["state"],
  "message": ["note"],
  "strict": false,
  "replaceDefaults": false
}
```

- `replaceDefaults=false` (default): custom aliases are tried first, then built-ins.
- `replaceDefaults=true`: only aliases from the profile are used.
- strict mode can be enabled in the profile (`"strict": true`) or via CLI `--strict`.

## Status

Early but usable (v0.x). Current focus:
- robust log normalization
- operator UX polish and command/module maintainability
- test coverage on parsing, summaries, and CLI output contracts

## Roadmap

- [x] Feed view for JSON/JSONL logs
- [x] Summary stats by agent/action/status
- [x] RFC3339 time-window filtering (`--since`, `--until`)
- [x] Relative time-window filtering (`--last 30m`)
- [x] Optional output formats (JSON/table)
- [x] Per-agent drill-down filter (`--agent`)
- [x] Per-source drill-down filter (`--source`)
- [x] Message substring filter (`--contains`)
- [x] Dedicated per-agent subcommand
- [x] Multi-file input merge (`--input a.jsonl,b.jsonl`)
- [x] Tail mode for recent feed slices (`--limit N --tail`)
- [x] Gzip-compressed JSONL input (`.jsonl.gz`)
- [x] Release automation (tag-triggered matrix builds + checksums + GitHub Releases)

## Reproducible CI

All workflows pin third-party actions to immutable commits and explicit tool versions to avoid supply-chain drift:

- `actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5`
- `actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff`
- `dominikh/staticcheck-action@e986ce0bb60df51c6c9f5ccbf853d1b5bd1ca14c` (`version: 2026.1`)
- `actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02`
- `actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093`
- `softprops/action-gh-release@a06a81a03ee405af7f2048a818ed3f03bbf83c7b`

Update these commits only after reviewing the upstream changelog and verifying compatibility.

## Releases

Pushing a tag matching `v*` (for example `v0.6.5`) triggers automated release publishing:
- cross-platform binaries (linux/darwin/windows)
- a single `checksums.txt` (sha256 for all artifacts)
- autogenerated GitHub release notes

See also: [`RELEASE_PLAN.md`](./RELEASE_PLAN.md) for v0.1–v0.3 milestones.

## License

MIT
