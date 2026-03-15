package main

import (
	"fmt"
)

func execute(args []string) error {
	if len(args) < 2 {
		usage()
		return nil
	}

	switch args[1] {
	case "feed":
		return runFeed(args[2:])
	case "stats":
		return runStats(args[2:])
	case "agent":
		return runAgent(args[2:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q", args[1])
	}
}

func usage() {
	fmt.Print(`swarmscope - multi-agent run log inspector

Usage:
  swarmscope feed  --input run.jsonl[,run2.jsonl] [--limit N] [--tail] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
  swarmscope stats --input run.jsonl[,run2.jsonl] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
  swarmscope agent --input run.jsonl[,run2.jsonl] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]

Notes:
  Use --input - to read one JSON/JSONL payload from stdin.
`)
}
