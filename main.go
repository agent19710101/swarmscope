package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

type Event struct {
	Time    time.Time
	Agent   string
	Action  string
	Status  string
	Message string
	Source  string
}

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
	case "help", "-h", "--help":
		usage()
	default:
		exitErr(fmt.Errorf("unknown subcommand %q", os.Args[1]))
	}
}

func runFeed(args []string) error {
	fs := flag.NewFlagSet("feed", flag.ContinueOnError)
	input := fs.String("input", "", "input JSON/JSONL file (required)")
	limit := fs.Int("limit", 0, "max events to print (0 = all)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("feed: --input is required")
	}
	events, err := loadEvents(*input)
	if err != nil {
		return err
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	if *limit > 0 && *limit < len(events) {
		events = events[:*limit]
	}

	for i, ev := range events {
		ts := ev.Time.Format("15:04:05")
		fmt.Printf("%03d %s  %-12s  %-12s  %-6s  %s\n", i+1, ts, truncate(ev.Agent, 12), truncate(ev.Action, 12), truncate(ev.Status, 6), truncate(ev.Message, 80))
	}
	return nil
}

func runStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	input := fs.String("input", "", "input JSON/JSONL file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("stats: --input is required")
	}
	events, err := loadEvents(*input)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		fmt.Println("no events found")
		return nil
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })

	agentCount := map[string]int{}
	actionCount := map[string]int{}
	statusCount := map[string]int{}

	for _, ev := range events {
		agentCount[ev.Agent]++
		actionCount[ev.Action]++
		statusCount[ev.Status]++
	}

	duration := events[len(events)-1].Time.Sub(events[0].Time)
	fmt.Printf("events:   %d\n", len(events))
	fmt.Printf("window:   %s -> %s (%s)\n", events[0].Time.Format(time.RFC3339), events[len(events)-1].Time.Format(time.RFC3339), duration)
	fmt.Println()
	printCountTable("agents", agentCount)
	fmt.Println()
	printCountTable("actions", actionCount)
	fmt.Println()
	printCountTable("status", statusCount)
	return nil
}

func loadEvents(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	events, err := decodeJSONL(f)
	if err == nil {
		return events, nil
	}
	if !errors.Is(err, io.EOF) {
		if events2, err2 := decodeJSONArray(path); err2 == nil {
			return events2, nil
		}
	}
	return nil, fmt.Errorf("parse input as JSONL/JSON array: %w", err)
}

func decodeJSONL(r io.Reader) ([]Event, error) {
	var events []Event
	s := bufio.NewScanner(r)
	line := 0
	for s.Scan() {
		line++
		text := strings.TrimSpace(s.Text())
		if text == "" {
			continue
		}
		ev, err := parseOne([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		events = append(events, ev)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, io.EOF
	}
	return events, nil
}

func decodeJSONArray(path string) ([]Event, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(raw))
	for i, m := range raw {
		bb, _ := json.Marshal(m)
		ev, err := parseOne(bb)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

func parseOne(line []byte) (Event, error) {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return Event{}, err
	}
	ev := Event{
		Time:    pickTime(m),
		Agent:   pickString(m, "agent", "agent_name", "worker", "session"),
		Action:  pickString(m, "action", "event", "type", "tool"),
		Status:  pickString(m, "status", "level", "result"),
		Message: pickString(m, "message", "msg", "summary", "content"),
	}
	if ev.Agent == "" {
		ev.Agent = "unknown"
	}
	if ev.Action == "" {
		ev.Action = "unknown"
	}
	if ev.Status == "" {
		ev.Status = "unknown"
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	return ev, nil
}

func pickTime(m map[string]any) time.Time {
	for _, k := range []string{"ts", "time", "timestamp", "created_at"} {
		v, ok := m[k]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
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

func usage() {
	fmt.Print(`swarmscope - multi-agent run log inspector

Usage:
  swarmscope feed  --input run.jsonl [--limit N]
  swarmscope stats --input run.jsonl
`)
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
