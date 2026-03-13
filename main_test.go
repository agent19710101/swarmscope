package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadEventsJSONL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "run.jsonl")
	content := `{"ts":"2026-03-13T01:10:00Z","agent":"a","action":"plan","status":"ok","message":"m1"}
{"timestamp":"2026-03-13T01:10:02Z","worker":"b","event":"test","result":"fail","summary":"m2"}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := loadEvents(p, defaultParserProfile())
	if err != nil {
		t.Fatalf("loadEvents error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[1].Agent != "b" || events[1].Action != "test" || events[1].Status != "fail" {
		t.Fatalf("unexpected normalization: %+v", events[1])
	}
}

func TestLoadEventsJSONLGzip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "run.jsonl.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	_, err = gz.Write([]byte("{\"ts\":\"2026-03-13T01:10:00Z\",\"agent\":\"a\",\"action\":\"plan\",\"status\":\"ok\",\"message\":\"m1\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	events, err := loadEvents(p, defaultParserProfile())
	if err != nil {
		t.Fatalf("loadEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Agent != "a" || events[0].Action != "plan" {
		t.Fatalf("unexpected normalization: %+v", events[0])
	}
}

func TestLoadEventsJSONLLargeLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "large.jsonl")
	largeMessage := strings.Repeat("x", 70*1024)
	content := "{\"ts\":\"2026-03-13T01:10:00Z\",\"agent\":\"a\",\"action\":\"plan\",\"status\":\"ok\",\"message\":\"" + largeMessage + "\"}\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := loadEvents(p, defaultParserProfile())
	if err != nil {
		t.Fatalf("loadEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if len(events[0].Message) != len(largeMessage) {
		t.Fatalf("unexpected message length: got %d, want %d", len(events[0].Message), len(largeMessage))
	}
}

func TestParseOneDefaults(t *testing.T) {
	ev, err := parseOne([]byte(`{"foo":"bar"}`), defaultParserProfile())
	if err != nil {
		t.Fatal(err)
	}
	if ev.Agent != "unknown" || ev.Action != "unknown" || ev.Status != "unknown" {
		t.Fatalf("unexpected defaults: %+v", ev)
	}
	if want := time.Unix(0, 0).UTC(); !ev.Time.Equal(want) {
		t.Fatalf("expected deterministic fallback time %s, got %s", want.Format(time.RFC3339), ev.Time.Format(time.RFC3339))
	}
}

func TestApplyTimeWindow(t *testing.T) {
	events := []Event{
		{Time: time.Date(2026, 3, 13, 1, 0, 0, 0, time.UTC), Agent: "a"},
		{Time: time.Date(2026, 3, 13, 1, 15, 0, 0, time.UTC), Agent: "b"},
		{Time: time.Date(2026, 3, 13, 1, 30, 0, 0, time.UTC), Agent: "c"},
	}

	got, err := applyTimeWindow(events, "2026-03-13T01:10:00Z", "2026-03-13T01:30:00Z")
	if err != nil {
		t.Fatalf("applyTimeWindow error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if got[0].Agent != "b" || got[1].Agent != "c" {
		t.Fatalf("unexpected events: %+v", got)
	}
}

func TestApplyTimeWindowErrors(t *testing.T) {
	events := []Event{{Time: time.Date(2026, 3, 13, 1, 0, 0, 0, time.UTC)}}

	if _, err := applyTimeWindow(events, "nope", ""); err == nil {
		t.Fatal("expected invalid since error")
	}
	if _, err := applyTimeWindow(events, "", "nope"); err == nil {
		t.Fatal("expected invalid until error")
	}
	if _, err := applyTimeWindow(events, "2026-03-13T02:00:00Z", "2026-03-13T01:00:00Z"); err == nil {
		t.Fatal("expected since > until error")
	}
}

func TestNormalizeTimeWindowArgsLast(t *testing.T) {
	now := time.Date(2026, 3, 13, 2, 30, 0, 0, time.UTC)
	since, until, err := normalizeTimeWindowArgs("", "", "30m", now)
	if err != nil {
		t.Fatalf("normalizeTimeWindowArgs error: %v", err)
	}
	if since != "2026-03-13T02:00:00Z" {
		t.Fatalf("unexpected since: %s", since)
	}
	if until != "2026-03-13T02:30:00Z" {
		t.Fatalf("unexpected until: %s", until)
	}
}

func TestNormalizeTimeWindowArgsErrors(t *testing.T) {
	now := time.Date(2026, 3, 13, 2, 30, 0, 0, time.UTC)

	if _, _, err := normalizeTimeWindowArgs("2026-03-13T02:00:00Z", "", "30m", now); err == nil {
		t.Fatal("expected conflict error for --last and --since")
	}
	if _, _, err := normalizeTimeWindowArgs("", "", "-5m", now); err == nil {
		t.Fatal("expected positive duration error")
	}
	if _, _, err := normalizeTimeWindowArgs("", "", "banana", now); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestBuildStats(t *testing.T) {
	events := []Event{
		{Time: time.Date(2026, 3, 13, 1, 0, 0, 0, time.UTC), Agent: "a", Action: "plan", Status: "ok"},
		{Time: time.Date(2026, 3, 13, 1, 0, 2, 0, time.UTC), Agent: "b", Action: "test", Status: "fail"},
		{Time: time.Date(2026, 3, 13, 1, 0, 5, 0, time.UTC), Agent: "a", Action: "edit", Status: "ok"},
	}

	got := buildStats(events)
	if got.Events != 3 {
		t.Fatalf("want 3 events, got %d", got.Events)
	}
	if got.Window.Start != "2026-03-13T01:00:00Z" || got.Window.End != "2026-03-13T01:00:05Z" {
		t.Fatalf("unexpected window: %+v", got.Window)
	}
	if got.Window.Duration != "5s" {
		t.Fatalf("unexpected duration: %s", got.Window.Duration)
	}
	if got.Agents["a"] != 2 || got.Actions["test"] != 1 || got.Status["ok"] != 2 {
		t.Fatalf("unexpected counts: %+v", got)
	}
}

func TestApplyAgentFilter(t *testing.T) {
	events := []Event{{Agent: "Planner"}, {Agent: "coder-a"}, {Agent: "reviewer"}}

	got := applyAgentFilter(events, "planner, reviewer")
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if got[0].Agent != "Planner" || got[1].Agent != "reviewer" {
		t.Fatalf("unexpected filtered events: %+v", got)
	}
}

func TestParseAgentSet(t *testing.T) {
	set := parseAgentSet(" planner, ,CODER-A  ")
	if len(set) != 2 {
		t.Fatalf("want 2 agents, got %d", len(set))
	}
	if _, ok := set["planner"]; !ok {
		t.Fatal("expected planner in set")
	}
	if _, ok := set["coder-a"]; !ok {
		t.Fatal("expected coder-a in set")
	}
}

func TestApplyContainsFilter(t *testing.T) {
	events := []Event{
		{Agent: "planner", Message: "Decomposed issue #42"},
		{Agent: "coder-a", Message: "updated parser"},
		{Agent: "reviewer", Message: "Requested edge-case fix"},
	}

	got := applyContainsFilter(events, "  ISSUE ")
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d", len(got))
	}
	if got[0].Agent != "planner" {
		t.Fatalf("unexpected match: %+v", got[0])
	}

	all := applyContainsFilter(events, " ")
	if len(all) != len(events) {
		t.Fatalf("blank needle should keep all events, got %d", len(all))
	}
}

func TestBuildAgentStats(t *testing.T) {
	events := []Event{
		{Time: time.Date(2026, 3, 13, 1, 0, 0, 0, time.UTC), Agent: "a", Action: "plan", Status: "ok"},
		{Time: time.Date(2026, 3, 13, 1, 0, 2, 0, time.UTC), Agent: "b", Action: "test", Status: "fail"},
		{Time: time.Date(2026, 3, 13, 1, 0, 5, 0, time.UTC), Agent: "a", Action: "edit", Status: "ok"},
		{Time: time.Date(2026, 3, 13, 1, 0, 6, 0, time.UTC), Agent: "a", Action: "edit", Status: "warn"},
	}

	got := buildAgentStats(events)
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0].Agent != "a" || got[0].Events != 3 {
		t.Fatalf("unexpected first row: %+v", got[0])
	}
	if got[0].FirstSeen != "2026-03-13T01:00:00Z" || got[0].LastSeen != "2026-03-13T01:00:06Z" {
		t.Fatalf("unexpected time bounds: %+v", got[0])
	}
	if got[0].Actions != 2 || got[0].Statuses != 2 {
		t.Fatalf("unexpected cardinality counts: %+v", got[0])
	}
	if got[1].Agent != "b" || got[1].Events != 1 {
		t.Fatalf("unexpected second row: %+v", got[1])
	}
}

func TestApplyLimit(t *testing.T) {
	events := []Event{{Agent: "a"}, {Agent: "b"}, {Agent: "c"}, {Agent: "d"}}

	if got := applyLimit(events, 0, false); len(got) != 4 {
		t.Fatalf("limit=0 should keep all events, got %d", len(got))
	}

	if got := applyLimit(events, 2, false); len(got) != 2 || got[0].Agent != "a" || got[1].Agent != "b" {
		t.Fatalf("unexpected head limit result: %+v", got)
	}

	if got := applyLimit(events, 2, true); len(got) != 2 || got[0].Agent != "c" || got[1].Agent != "d" {
		t.Fatalf("unexpected tail limit result: %+v", got)
	}
}

func TestParseInputPaths(t *testing.T) {
	paths, err := parseInputPaths(" ./a.jsonl,./b.jsonl , ")
	if err != nil {
		t.Fatalf("parseInputPaths error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 paths, got %d", len(paths))
	}
	if paths[0] != "./a.jsonl" || paths[1] != "./b.jsonl" {
		t.Fatalf("unexpected paths: %+v", paths)
	}

	if _, err := parseInputPaths(" , "); err == nil {
		t.Fatal("expected error for empty input list")
	}
}

func TestLoadEventsFromPaths(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "one.jsonl")
	p2 := filepath.Join(dir, "two.jsonl")

	if err := os.WriteFile(p1, []byte("{\"ts\":\"2026-03-13T01:10:00Z\",\"agent\":\"a\",\"action\":\"plan\",\"status\":\"ok\",\"message\":\"m1\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("{\"ts\":\"2026-03-13T01:10:01Z\",\"agent\":\"b\",\"action\":\"test\",\"status\":\"ok\",\"message\":\"m2\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := loadEventsFromPaths([]string{p1, p2}, defaultParserProfile())
	if err != nil {
		t.Fatalf("loadEventsFromPaths error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].Source != p1 || events[1].Source != p2 {
		t.Fatalf("unexpected source fields: %+v", events)
	}
}

func TestLoadEventsCustomProfileLenient(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.jsonl")
	content := "{\"when\":\"2026-03-13T01:10:00Z\",\"actor\":\"coder\",\"op\":\"edit\",\"state\":\"ok\",\"note\":\"done\"}\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := defaultParserProfile()
	profile.TimestampKeys = append([]string{"when"}, profile.TimestampKeys...)
	profile.AgentKeys = append([]string{"actor"}, profile.AgentKeys...)
	profile.ActionKeys = append([]string{"op"}, profile.ActionKeys...)
	profile.StatusKeys = append([]string{"state"}, profile.StatusKeys...)
	profile.MessageKeys = append([]string{"note"}, profile.MessageKeys...)

	events, err := loadEvents(p, profile)
	if err != nil {
		t.Fatalf("loadEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Agent != "coder" || events[0].Action != "edit" || events[0].Status != "ok" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestLoadEventsStrictMode(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "strict.jsonl")
	if err := os.WriteFile(p, []byte("{\"agent\":\"a\",\"action\":\"plan\",\"status\":\"ok\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := defaultParserProfile()
	profile.Strict = true
	if _, err := loadEvents(p, profile); err == nil {
		t.Fatal("expected strict mode to fail when timestamp is missing")
	}
}
