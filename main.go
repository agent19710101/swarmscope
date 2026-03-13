package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "feed":
		if err := runFeed(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "stats":
		if err := runStats(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "agent":
		if err := runAgent(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "help", "-h", "--help":
		usage()
	default:
		exitErr(fmt.Errorf("unknown subcommand %q", os.Args[1]))
	}
}

func usage() {
	fmt.Print(`swarmscope - multi-agent run log inspector

Usage:
  swarmscope feed  --input run.jsonl[,run2.jsonl] [--limit N] [--tail] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
  swarmscope stats --input run.jsonl[,run2.jsonl] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
  swarmscope agent --input run.jsonl[,run2.jsonl] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
`)
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
