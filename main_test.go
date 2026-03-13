package main

import (
	"os"
	"path/filepath"
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
	events, err := loadEvents(p)
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

func TestParseOneDefaults(t *testing.T) {
	ev, err := parseOne([]byte(`{"foo":"bar"}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Agent != "unknown" || ev.Action != "unknown" || ev.Status != "unknown" {
		t.Fatalf("unexpected defaults: %+v", ev)
	}
	if ev.Time.IsZero() {
		t.Fatal("expected fallback time")
	}
	if time.Since(ev.Time) > 2*time.Second {
		t.Fatal("fallback time too old")
	}
}
