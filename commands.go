package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Command handlers own argument parsing and delegate logic to focused helpers.
func runFeed(args []string) error {
	fs := flag.NewFlagSet("feed", flag.ContinueOnError)
	input := fs.String("input", "", "input JSON/JSONL file(s), comma-separated (required)")
	limit := fs.Int("limit", 0, "max events to print (0 = all)")
	tail := fs.Bool("tail", false, "when used with --limit, print most recent N events instead of oldest N")
	format := fs.String("format", "table", "output format: table|json")
	agent := fs.String("agent", "", "filter by agent name (comma-separated for multiple)")
	source := fs.String("source", "", "filter by source file path or basename (comma-separated for multiple)")
	contains := fs.String("contains", "", "only include events whose message contains this case-insensitive substring")
	since := fs.String("since", "", "only include events at or after RFC3339 timestamp")
	until := fs.String("until", "", "only include events at or before RFC3339 timestamp")
	last := fs.String("last", "", "only include events from the most recent duration (e.g. 30m, 2h)")
	mapPath := fs.String("map", "", "optional JSON field-mapping profile path")
	strict := fs.Bool("strict", false, "strict mode: fail when canonical fields cannot be resolved")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("feed: --input is required")
	}
	inputs, err := parseInputPaths(*input)
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	profile, err := loadParserProfile(*mapPath, *strict, boolFlagWasProvided(fs, "strict"))
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	events, err := loadEventsFromPaths(inputs, profile)
	if err != nil {
		return err
	}
	sinceRaw, untilRaw, err := normalizeTimeWindowArgs(*since, *until, *last, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	events, err = applyTimeWindow(events, sinceRaw, untilRaw)
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	events = applyAgentFilter(events, *agent)
	events = applySourceFilter(events, *source)
	events = applyContainsFilter(events, *contains)
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	events = applyLimit(events, *limit, *tail)

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "table":
		for i, ev := range events {
			ts := ev.Time.Format("15:04:05")
			fmt.Printf("%03d %s  %-12s  %-12s  %-6s  %s\n", i+1, ts, truncate(ev.Agent, 12), truncate(ev.Action, 12), truncate(ev.Status, 6), truncate(ev.Message, 80))
		}
		return nil
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	default:
		return fmt.Errorf("feed: unsupported --format %q (want table|json)", *format)
	}
}

func runStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	input := fs.String("input", "", "input JSON/JSONL file(s), comma-separated (required)")
	format := fs.String("format", "table", "output format: table|json")
	agent := fs.String("agent", "", "filter by agent name (comma-separated for multiple)")
	source := fs.String("source", "", "filter by source file path or basename (comma-separated for multiple)")
	contains := fs.String("contains", "", "only include events whose message contains this case-insensitive substring")
	since := fs.String("since", "", "only include events at or after RFC3339 timestamp")
	until := fs.String("until", "", "only include events at or before RFC3339 timestamp")
	last := fs.String("last", "", "only include events from the most recent duration (e.g. 30m, 2h)")
	mapPath := fs.String("map", "", "optional JSON field-mapping profile path")
	strict := fs.Bool("strict", false, "strict mode: fail when canonical fields cannot be resolved")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("stats: --input is required")
	}
	inputs, err := parseInputPaths(*input)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	profile, err := loadParserProfile(*mapPath, *strict, boolFlagWasProvided(fs, "strict"))
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	events, err := loadEventsFromPaths(inputs, profile)
	if err != nil {
		return err
	}
	sinceRaw, untilRaw, err := normalizeTimeWindowArgs(*since, *until, *last, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	events, err = applyTimeWindow(events, sinceRaw, untilRaw)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	events = applyAgentFilter(events, *agent)
	events = applySourceFilter(events, *source)
	events = applyContainsFilter(events, *contains)
	if len(events) == 0 {
		if strings.EqualFold(strings.TrimSpace(*format), "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(statsOutput{})
		}
		fmt.Println("no events found")
		return nil
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })

	summary := buildStats(events)

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "table":
		fmt.Printf("events:   %d\n", summary.Events)
		fmt.Printf("window:   %s -> %s (%s)\n", summary.Window.Start, summary.Window.End, summary.Window.Duration)
		fmt.Println()
		printCountTable("agents", summary.Agents)
		fmt.Println()
		printCountTable("actions", summary.Actions)
		fmt.Println()
		printCountTable("status", summary.Status)
		return nil
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	default:
		return fmt.Errorf("stats: unsupported --format %q (want table|json)", *format)
	}
}

func runAgent(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	input := fs.String("input", "", "input JSON/JSONL file(s), comma-separated (required)")
	format := fs.String("format", "table", "output format: table|json")
	agent := fs.String("agent", "", "filter by agent name (comma-separated for multiple)")
	source := fs.String("source", "", "filter by source file path or basename (comma-separated for multiple)")
	contains := fs.String("contains", "", "only include events whose message contains this case-insensitive substring")
	since := fs.String("since", "", "only include events at or after RFC3339 timestamp")
	until := fs.String("until", "", "only include events at or before RFC3339 timestamp")
	last := fs.String("last", "", "only include events from the most recent duration (e.g. 30m, 2h)")
	mapPath := fs.String("map", "", "optional JSON field-mapping profile path")
	strict := fs.Bool("strict", false, "strict mode: fail when canonical fields cannot be resolved")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("agent: --input is required")
	}
	inputs, err := parseInputPaths(*input)
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}

	profile, err := loadParserProfile(*mapPath, *strict, boolFlagWasProvided(fs, "strict"))
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	events, err := loadEventsFromPaths(inputs, profile)
	if err != nil {
		return err
	}
	sinceRaw, untilRaw, err := normalizeTimeWindowArgs(*since, *until, *last, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	events, err = applyTimeWindow(events, sinceRaw, untilRaw)
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	events = applyAgentFilter(events, *agent)
	events = applySourceFilter(events, *source)
	events = applyContainsFilter(events, *contains)

	summary := buildAgentStats(events)
	if len(summary) == 0 {
		if strings.EqualFold(strings.TrimSpace(*format), "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode([]agentOutput{})
		}
		fmt.Println("no events found")
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "table":
		fmt.Println("agents:")
		for _, row := range summary {
			fmt.Printf("  %-18s %4d events  first=%s  last=%s  actions=%d  statuses=%d\n",
				truncate(row.Agent, 18), row.Events, row.FirstSeen, row.LastSeen, row.Actions, row.Statuses)
		}
		return nil
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	default:
		return fmt.Errorf("agent: unsupported --format %q (want table|json)", *format)
	}
}
