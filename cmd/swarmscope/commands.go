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

	"github.com/agent19710101/swarmscope/internal/feed"
	"github.com/agent19710101/swarmscope/internal/ingest"
	"github.com/agent19710101/swarmscope/internal/stats"
)

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
	skipInvalid := fs.Bool("skip-invalid", false, "skip malformed records and report how many were ignored")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("feed: --input is required")
	}
	inputs, err := ingest.ParseInputPaths(*input)
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	profile, err := ingest.LoadParserProfile(*mapPath, *strict, flagWasProvided(fs, "strict"))
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	report, err := ingest.LoadEventsReportFromPaths(inputs, profile, *skipInvalid)
	if err != nil {
		return err
	}
	events := report.Events
	sinceRaw, untilRaw, err := feed.NormalizeTimeWindowArgs(*since, *until, *last, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	events, err = feed.ApplyTimeWindow(events, sinceRaw, untilRaw)
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	events = feed.ApplyAgentFilter(events, *agent)
	events = feed.ApplySourceFilter(events, *source)
	events = feed.ApplyContainsFilter(events, *contains)
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	events = feed.ApplyLimit(events, *limit, *tail)

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "table":
		for i, ev := range events {
			ts := ev.Time.Format("15:04:05")
			fmt.Printf("%03d %s  %-12s  %-12s  %-6s  %s\n", i+1, ts, truncate(ev.Agent, 12), truncate(ev.Action, 12), truncate(ev.Status, 6), truncate(ev.Message, 80))
		}
		if report.Skipped > 0 {
			fmt.Printf("\nignored %d malformed record(s) via --skip-invalid\n", report.Skipped)
		}
		return nil
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if *skipInvalid {
			report.Events = events
			return enc.Encode(report)
		}
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
	skipInvalid := fs.Bool("skip-invalid", false, "skip malformed records and report how many were ignored")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("stats: --input is required")
	}
	inputs, err := ingest.ParseInputPaths(*input)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	profile, err := ingest.LoadParserProfile(*mapPath, *strict, flagWasProvided(fs, "strict"))
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	report, err := ingest.LoadEventsReportFromPaths(inputs, profile, *skipInvalid)
	if err != nil {
		return err
	}
	events := report.Events
	sinceRaw, untilRaw, err := feed.NormalizeTimeWindowArgs(*since, *until, *last, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	events, err = feed.ApplyTimeWindow(events, sinceRaw, untilRaw)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	events = feed.ApplyAgentFilter(events, *agent)
	events = feed.ApplySourceFilter(events, *source)
	events = feed.ApplyContainsFilter(events, *contains)
	if len(events) == 0 {
		if strings.EqualFold(strings.TrimSpace(*format), "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if *skipInvalid {
				return enc.Encode(stats.Output{Summary: stats.Summary{}, Skipped: report.Skipped, Diagnostics: report.Diagnostics})
			}
			return enc.Encode(stats.Summary{})
		}
		fmt.Println("no events found")
		if report.Skipped > 0 {
			fmt.Printf("ignored %d malformed record(s) via --skip-invalid\n", report.Skipped)
		}
		return nil
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })

	summary := stats.BuildSummary(events)

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "table":
		fmt.Printf("events:   %d\n", summary.Events)
		fmt.Printf("window:   %s -> %s (%s)\n", summary.Window.Start, summary.Window.End, summary.Window.Duration)
		if report.Skipped > 0 {
			fmt.Printf("ignored:  %d malformed record(s)\n", report.Skipped)
		}
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
		if *skipInvalid {
			return enc.Encode(stats.Output{Summary: summary, Skipped: report.Skipped, Diagnostics: report.Diagnostics})
		}
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
	skipInvalid := fs.Bool("skip-invalid", false, "skip malformed records and report how many were ignored")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("agent: --input is required")
	}
	inputs, err := ingest.ParseInputPaths(*input)
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	profile, err := ingest.LoadParserProfile(*mapPath, *strict, flagWasProvided(fs, "strict"))
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	report, err := ingest.LoadEventsReportFromPaths(inputs, profile, *skipInvalid)
	if err != nil {
		return err
	}
	events := report.Events
	sinceRaw, untilRaw, err := feed.NormalizeTimeWindowArgs(*since, *until, *last, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	events, err = feed.ApplyTimeWindow(events, sinceRaw, untilRaw)
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	events = feed.ApplyAgentFilter(events, *agent)
	events = feed.ApplySourceFilter(events, *source)
	events = feed.ApplyContainsFilter(events, *contains)

	summary := stats.BuildAgentSummaries(events)
	if len(summary) == 0 {
		if strings.EqualFold(strings.TrimSpace(*format), "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if *skipInvalid {
				return enc.Encode(stats.AgentOutput{Agents: []stats.AgentSummary{}, Skipped: report.Skipped, Diagnostics: report.Diagnostics})
			}
			return enc.Encode([]stats.AgentSummary{})
		}
		fmt.Println("no events found")
		if report.Skipped > 0 {
			fmt.Printf("ignored %d malformed record(s) via --skip-invalid\n", report.Skipped)
		}
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "table":
		fmt.Println("agents:")
		for _, row := range summary {
			fmt.Printf("  %-18s %4d events  first=%s  last=%s  actions=%d  statuses=%d\n",
				truncate(row.Agent, 18), row.Events, row.FirstSeen, row.LastSeen, row.Actions, row.Statuses)
		}
		if report.Skipped > 0 {
			fmt.Printf("\nignored %d malformed record(s) via --skip-invalid\n", report.Skipped)
		}
		return nil
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if *skipInvalid {
			return enc.Encode(stats.AgentOutput{Agents: summary, Skipped: report.Skipped, Diagnostics: report.Diagnostics})
		}
		return enc.Encode(summary)
	default:
		return fmt.Errorf("agent: unsupported --format %q (want table|json)", *format)
	}
}

func printCountTable(title string, counts map[string]int) {
	type row struct {
		key string
		n   int
	}
	rows := make([]row, 0, len(counts))
	for k, n := range counts {
		rows = append(rows, row{key: k, n: n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n == rows[j].n {
			return rows[i].key < rows[j].key
		}
		return rows[i].n > rows[j].n
	})
	fmt.Println(title + ":")
	for _, r := range rows {
		fmt.Printf("  %-18s %4d\n", truncate(r.key, 18), r.n)
	}
}
func flagWasProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
