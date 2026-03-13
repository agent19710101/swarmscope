package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type parserProfile struct {
	TimestampKeys  []string
	AgentKeys      []string
	ActionKeys     []string
	StatusKeys     []string
	MessageKeys    []string
	Strict         bool
	ReplaceDefault bool
}

type profileFile struct {
	Timestamp      []string `json:"timestamp"`
	Agent          []string `json:"agent"`
	Action         []string `json:"action"`
	Status         []string `json:"status"`
	Message        []string `json:"message"`
	Strict         *bool    `json:"strict"`
	ReplaceDefault bool     `json:"replaceDefaults"`
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
	profile, err := loadParserProfile(*mapPath, *strict)
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
	profile, err := loadParserProfile(*mapPath, *strict)
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

type timeWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration string `json:"duration"`
}

type statsOutput struct {
	Events  int            `json:"events"`
	Window  timeWindow     `json:"window"`
	Agents  map[string]int `json:"agents"`
	Actions map[string]int `json:"actions"`
	Status  map[string]int `json:"status"`
}

type agentOutput struct {
	Agent     string `json:"agent"`
	Events    int    `json:"events"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Actions   int    `json:"actions"`
	Statuses  int    `json:"statuses"`
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

	profile, err := loadParserProfile(*mapPath, *strict)
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

func buildStats(events []Event) statsOutput {
	agentCount := map[string]int{}
	actionCount := map[string]int{}
	statusCount := map[string]int{}

	for _, ev := range events {
		agentCount[ev.Agent]++
		actionCount[ev.Action]++
		statusCount[ev.Status]++
	}

	duration := events[len(events)-1].Time.Sub(events[0].Time)
	return statsOutput{
		Events: len(events),
		Window: timeWindow{
			Start:    events[0].Time.Format(time.RFC3339),
			End:      events[len(events)-1].Time.Format(time.RFC3339),
			Duration: duration.String(),
		},
		Agents:  agentCount,
		Actions: actionCount,
		Status:  statusCount,
	}
}

func buildAgentStats(events []Event) []agentOutput {
	type acc struct {
		events   int
		first    time.Time
		last     time.Time
		actions  map[string]struct{}
		statuses map[string]struct{}
	}

	byAgent := map[string]*acc{}
	for _, ev := range events {
		entry, ok := byAgent[ev.Agent]
		if !ok {
			entry = &acc{first: ev.Time, last: ev.Time, actions: map[string]struct{}{}, statuses: map[string]struct{}{}}
			byAgent[ev.Agent] = entry
		}
		entry.events++
		if ev.Time.Before(entry.first) {
			entry.first = ev.Time
		}
		if ev.Time.After(entry.last) {
			entry.last = ev.Time
		}
		entry.actions[ev.Action] = struct{}{}
		entry.statuses[ev.Status] = struct{}{}
	}

	out := make([]agentOutput, 0, len(byAgent))
	for agent, entry := range byAgent {
		out = append(out, agentOutput{
			Agent:     agent,
			Events:    entry.events,
			FirstSeen: entry.first.Format(time.RFC3339),
			LastSeen:  entry.last.Format(time.RFC3339),
			Actions:   len(entry.actions),
			Statuses:  len(entry.statuses),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Events == out[j].Events {
			return out[i].Agent < out[j].Agent
		}
		return out[i].Events > out[j].Events
	})
	return out
}

func defaultParserProfile() parserProfile {
	return parserProfile{
		TimestampKeys: []string{"ts", "time", "timestamp", "created_at"},
		AgentKeys:     []string{"agent", "agent_name", "worker", "session"},
		ActionKeys:    []string{"action", "event", "type", "tool"},
		StatusKeys:    []string{"status", "level", "result"},
		MessageKeys:   []string{"message", "msg", "summary", "content"},
	}
}

func loadParserProfile(path string, strict bool) (parserProfile, error) {
	profile := defaultParserProfile()
	profile.Strict = strict
	if strings.TrimSpace(path) == "" {
		return profile, nil
	}

	bb, err := os.ReadFile(path)
	if err != nil {
		return parserProfile{}, fmt.Errorf("read map profile: %w", err)
	}
	var cfg profileFile
	if err := json.Unmarshal(bb, &cfg); err != nil {
		return parserProfile{}, fmt.Errorf("parse map profile JSON: %w", err)
	}

	profile.ReplaceDefault = cfg.ReplaceDefault
	profile.TimestampKeys = mergeKeys(profile.TimestampKeys, cfg.Timestamp, cfg.ReplaceDefault)
	profile.AgentKeys = mergeKeys(profile.AgentKeys, cfg.Agent, cfg.ReplaceDefault)
	profile.ActionKeys = mergeKeys(profile.ActionKeys, cfg.Action, cfg.ReplaceDefault)
	profile.StatusKeys = mergeKeys(profile.StatusKeys, cfg.Status, cfg.ReplaceDefault)
	profile.MessageKeys = mergeKeys(profile.MessageKeys, cfg.Message, cfg.ReplaceDefault)
	if cfg.Strict != nil {
		profile.Strict = *cfg.Strict
	}
	return profile, nil
}

func mergeKeys(defaults, custom []string, replace bool) []string {
	if len(custom) == 0 {
		return defaults
	}
	if replace {
		return sanitizeKeys(custom)
	}
	merged := append([]string{}, custom...)
	merged = append(merged, defaults...)
	return sanitizeKeys(merged)
}

func sanitizeKeys(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, key := range in {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func parseInputPaths(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		out = append(out, path)
	}
	if len(out) == 0 {
		return nil, errors.New("--input must include at least one file path")
	}
	return out, nil
}

func loadEventsFromPaths(paths []string, profile parserProfile) ([]Event, error) {
	all := make([]Event, 0)
	for _, path := range paths {
		events, err := loadEvents(path, profile)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", path, err)
		}
		for i := range events {
			events[i].Source = path
		}
		all = append(all, events...)
	}
	return all, nil
}

func loadEvents(path string, profile parserProfile) ([]Event, error) {
	r, err := openInputReader(path)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	events, err := decodeJSONL(r, profile)
	_ = r.Close()
	if err == nil {
		return events, nil
	}

	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse input as JSONL/JSON array: %w", err)
	}

	r2, err2 := openInputReader(path)
	if err2 != nil {
		return nil, fmt.Errorf("open input: %w", err2)
	}
	defer r2.Close()
	events2, err2 := decodeJSONArray(r2, profile)
	if err2 == nil {
		return events2, nil
	}
	return nil, fmt.Errorf("parse input as JSONL/JSON array: %w", err)
}

func decodeJSONL(r io.Reader, profile parserProfile) ([]Event, error) {
	var events []Event
	s := bufio.NewScanner(r)
	const maxJSONLLineBytes = 10 * 1024 * 1024
	s.Buffer(make([]byte, 0, 64*1024), maxJSONLLineBytes)
	line := 0
	for s.Scan() {
		line++
		text := strings.TrimSpace(s.Text())
		if text == "" {
			continue
		}
		ev, err := parseOne([]byte(text), profile)
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

func decodeJSONArray(r io.Reader, profile parserProfile) ([]Event, error) {
	var raw []map[string]any
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(raw))
	for i, m := range raw {
		bb, _ := json.Marshal(m)
		ev, err := parseOne(bb, profile)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

func openInputReader(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		return &multiReadCloser{Reader: gz, closers: []io.Closer{gz, f}}, nil
	}
	return f, nil
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiReadCloser) Close() error {
	var firstErr error
	for _, c := range m.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func parseOne(line []byte, profile parserProfile) (Event, error) {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return Event{}, err
	}
	ev := Event{
		Time:    pickTime(m, profile.TimestampKeys),
		Agent:   pickString(m, profile.AgentKeys...),
		Action:  pickString(m, profile.ActionKeys...),
		Status:  pickString(m, profile.StatusKeys...),
		Message: pickString(m, profile.MessageKeys...),
	}

	if profile.Strict {
		if ev.Time.IsZero() {
			return Event{}, errors.New("missing timestamp field")
		}
		if ev.Agent == "" {
			return Event{}, errors.New("missing agent field")
		}
		if ev.Action == "" {
			return Event{}, errors.New("missing action field")
		}
		if ev.Status == "" {
			return Event{}, errors.New("missing status field")
		}
	} else {
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
			ev.Time = time.Unix(0, 0).UTC()
		}
	}
	return ev, nil
}

func pickTime(m map[string]any, keys []string) time.Time {
	for _, k := range keys {
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

func normalizeTimeWindowArgs(sinceRaw, untilRaw, lastRaw string, now time.Time) (string, string, error) {
	sinceRaw = strings.TrimSpace(sinceRaw)
	untilRaw = strings.TrimSpace(untilRaw)
	lastRaw = strings.TrimSpace(lastRaw)

	if lastRaw == "" {
		return sinceRaw, untilRaw, nil
	}
	if sinceRaw != "" || untilRaw != "" {
		return "", "", errors.New("--last cannot be combined with --since or --until")
	}

	d, err := time.ParseDuration(lastRaw)
	if err != nil {
		return "", "", fmt.Errorf("invalid --last value %q: %w", lastRaw, err)
	}
	if d <= 0 {
		return "", "", errors.New("--last must be a positive duration")
	}

	end := now.UTC()
	start := end.Add(-d)
	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

func applyTimeWindow(events []Event, sinceRaw, untilRaw string) ([]Event, error) {
	var since, until time.Time
	var err error

	if strings.TrimSpace(sinceRaw) != "" {
		since, err = time.Parse(time.RFC3339, sinceRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid --since value %q: %w", sinceRaw, err)
		}
		since = since.UTC()
	}

	if strings.TrimSpace(untilRaw) != "" {
		until, err = time.Parse(time.RFC3339, untilRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid --until value %q: %w", untilRaw, err)
		}
		until = until.UTC()
	}

	if !since.IsZero() && !until.IsZero() && since.After(until) {
		return nil, errors.New("--since cannot be later than --until")
	}

	out := events[:0]
	for _, ev := range events {
		if !since.IsZero() && ev.Time.Before(since) {
			continue
		}
		if !until.IsZero() && ev.Time.After(until) {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func applyAgentFilter(events []Event, raw string) []Event {
	agents := parseAgentSet(raw)
	if len(agents) == 0 {
		return events
	}

	out := events[:0]
	for _, ev := range events {
		if _, ok := agents[strings.ToLower(ev.Agent)]; ok {
			out = append(out, ev)
		}
	}
	return out
}

func applySourceFilter(events []Event, raw string) []Event {
	sources := parseSourceSet(raw)
	if len(sources) == 0 {
		return events
	}

	out := events[:0]
	for _, ev := range events {
		source := strings.ToLower(strings.TrimSpace(ev.Source))
		base := strings.ToLower(filepath.Base(strings.TrimSpace(ev.Source)))
		if _, ok := sources[source]; ok {
			out = append(out, ev)
			continue
		}
		if _, ok := sources[base]; ok {
			out = append(out, ev)
		}
	}
	return out
}

func applyContainsFilter(events []Event, raw string) []Event {
	needle := strings.ToLower(strings.TrimSpace(raw))
	if needle == "" {
		return events
	}

	out := events[:0]
	for _, ev := range events {
		if strings.Contains(strings.ToLower(ev.Message), needle) {
			out = append(out, ev)
		}
	}
	return out
}

func parseSourceSet(raw string) map[string]struct{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			out[name] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseAgentSet(raw string) map[string]struct{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			out[name] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyLimit(events []Event, limit int, tail bool) []Event {
	if limit <= 0 || limit >= len(events) {
		return events
	}
	if tail {
		return events[len(events)-limit:]
	}
	return events[:limit]
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
  swarmscope feed  --input run.jsonl[,run2.jsonl] [--limit N] [--tail] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
  swarmscope stats --input run.jsonl[,run2.jsonl] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
  swarmscope agent --input run.jsonl[,run2.jsonl] [--format table|json] [--agent NAME[,NAME...]] [--source PATH[,PATH...]] [--contains TEXT] [--since RFC3339] [--until RFC3339] [--last 30m] [--map profile.json] [--strict]
`)
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
