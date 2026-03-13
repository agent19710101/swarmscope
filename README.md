# swarmscope

`swarmscope` is a Go CLI for **watching and summarizing multi-agent run logs** from mixed JSON/JSONL sources.

It helps answer:
- What did each agent do, in order?
- Which tools/actions dominate a run?
- Where did failures happen?

## Why

Agent orchestration is hot, but logs are fragmented and noisy. `swarmscope` gives you one terminal-first feed and summary for fast debugging, incident triage, and demos.

## Install

```bash
go install github.com/agent19710101/swarmscope@latest
```

## Usage

```bash
# unified chronological feed
swarmscope feed --input ./examples/run.jsonl

# focus only on a time window
swarmscope feed --input ./examples/run.jsonl \
  --since 2026-03-13T01:10:03Z --until 2026-03-13T01:10:09Z

# or use a relative window from "now"
swarmscope stats --input ./examples/run.jsonl --last 30m

# drill down to one (or more) agents
swarmscope feed --input ./examples/run.jsonl --agent coder-a
swarmscope stats --input ./examples/run.jsonl --agent coder-a,reviewer

# summary table by agent + action + status
swarmscope stats --input ./examples/run.jsonl

# per-agent activity summary (events, first/last seen, unique actions/statuses)
swarmscope agent --input ./examples/run.jsonl

# machine-readable output for piping/automation
swarmscope feed --input ./examples/run.jsonl --format json
swarmscope stats --input ./examples/run.jsonl --format json
swarmscope agent --input ./examples/run.jsonl --format json
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

- timestamp: `ts`, `time`, `timestamp`, `created_at`
- agent: `agent`, `agent_name`, `worker`, `session`
- action: `action`, `event`, `type`, `tool`
- status: `status`, `level`, `result`
- message: `message`, `msg`, `summary`, `content`

## Status

Early but usable (v0.x). Current focus:
- robust log normalization
- better CLI filters (absolute RFC3339 + relative `--last` windows, per-agent drill-down)
- test coverage on parsing and summaries

## Roadmap

- [x] Feed view for JSON/JSONL logs
- [x] Summary stats by agent/action/status
- [x] RFC3339 time-window filtering (`--since`, `--until`)
- [x] Relative time-window filtering (`--last 30m`)
- [x] Optional output formats (JSON/table)
- [x] Per-agent drill-down filter (`--agent`)
- [x] Dedicated per-agent subcommand
- [ ] Release automation

## License

MIT
