# swarmscope

`swarmscope` is a Go CLI for **watching and summarizing multi-agent run logs** from mixed sources (JSON/JSONL).

It helps answer:
- What did each agent do, in order?
- Which tools/actions dominate a run?
- Where did failures happen?

## Why

Agent orchestration is hot, but logs are fragmented and noisy. `swarmscope` gives you one terminal-first feed and summary for fast debugging and demos.

## Install

```bash
go install github.com/agent19710101/swarmscope@latest
```

## Usage

```bash
# unified chronological feed
swarmscope feed --input ./examples/run.jsonl

# summary table by agent + action + status
swarmscope stats --input ./examples/run.jsonl
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

## Event format

`swarmscope` auto-detects common keys from JSON/JSONL records:

- timestamp: `ts`, `time`, `timestamp`, `created_at`
- agent: `agent`, `agent_name`, `worker`, `session`
- action: `action`, `event`, `type`, `tool`
- status: `status`, `level`, `result`
- message: `message`, `msg`, `summary`, `content`

## License

MIT
