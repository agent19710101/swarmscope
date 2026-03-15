package ingest

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agent19710101/swarmscope/internal/model"
)

// ParseInputPaths splits a comma-separated --input value into individual paths.
func ParseInputPaths(raw string) ([]string, error) {
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

// LoadParserProfile reads a JSON map profile and merges it with the defaults.
func LoadParserProfile(path string, strict bool, strictSet bool) (Profile, error) {
	profile := defaultProfile()
	if strings.TrimSpace(path) == "" {
		return profile, nil
	}

	bb, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read map profile: %w", err)
	}
	var cfg profileFile
	if err := json.Unmarshal(bb, &cfg); err != nil {
		return Profile{}, fmt.Errorf("parse map profile JSON: %w", err)
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
	if strictSet {
		profile.Strict = strict
	}
	return profile, nil
}

// LoadEventsFromPaths reads each path, applies the parser profile, and annotates the source file.
func LoadEventsFromPaths(paths []string, profile Profile) ([]model.Event, error) {
	all := make([]model.Event, 0)
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

func defaultProfile() Profile {
	return Profile{
		TimestampKeys: []string{"ts", "time", "timestamp", "created_at"},
		AgentKeys:     []string{"agent", "agent_name", "worker", "session"},
		ActionKeys:    []string{"action", "event", "type", "tool"},
		StatusKeys:    []string{"status", "level", "result"},
		MessageKeys:   []string{"message", "msg", "summary", "content"},
	}
}

func loadEvents(path string, profile Profile) ([]model.Event, error) {
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
	return nil, fmt.Errorf("parse input as JSONL/JSON array: jsonl error: %v; json array error: %w", err, err2)
}

func decodeJSONL(r io.Reader, profile Profile) ([]model.Event, error) {
	var events []model.Event
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

func decodeJSONArray(r io.Reader, profile Profile) ([]model.Event, error) {
	var raw []map[string]any
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}
	events := make([]model.Event, 0, len(raw))
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

func parseOne(line []byte, profile Profile) (model.Event, error) {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return model.Event{}, err
	}
	ev := model.Event{
		Time:    pickTime(m, profile.TimestampKeys),
		Agent:   pickString(m, profile.AgentKeys...),
		Action:  pickString(m, profile.ActionKeys...),
		Status:  pickString(m, profile.StatusKeys...),
		Message: pickString(m, profile.MessageKeys...),
	}

	if profile.Strict {
		if ev.Time.IsZero() {
			return model.Event{}, errors.New("missing timestamp field")
		}
		if ev.Agent == "" {
			return model.Event{}, errors.New("missing agent field")
		}
		if ev.Action == "" {
			return model.Event{}, errors.New("missing action field")
		}
		if ev.Status == "" {
			return model.Event{}, errors.New("missing status field")
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
		switch vv := v.(type) {
		case string:
			ss := strings.TrimSpace(vv)
			if ss == "" {
				continue
			}
			if t, err := time.Parse(time.RFC3339, ss); err == nil {
				return t.UTC()
			}
			if nInt, err := strconv.ParseInt(ss, 10, 64); err == nil {
				if t := parseUnixTimestampInt(nInt); !t.IsZero() {
					return t
				}
			}
			if n, err := strconv.ParseFloat(ss, 64); err == nil {
				if t := parseUnixTimestamp(n); !t.IsZero() {
					return t
				}
			}
		case float64:
			if t := parseUnixTimestamp(vv); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}

func parseUnixTimestamp(v float64) time.Time {
	if math.IsNaN(v) || math.IsInf(v, 0) || v == 0 {
		return time.Time{}
	}
	if math.Trunc(v) == v {
		return parseUnixTimestampInt(int64(v))
	}

	abs := math.Abs(v)
	if abs >= 1e12 {
		secs := v / 1e3
		whole, frac := math.Modf(secs)
		return time.Unix(int64(whole), int64(frac*float64(time.Second))).UTC()
	}
	whole, frac := math.Modf(v)
	return time.Unix(int64(whole), int64(frac*float64(time.Second))).UTC()
}

func parseUnixTimestampInt(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	abs := v
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1e18:
		return time.Unix(0, v).UTC()
	case abs >= 1e15:
		return time.Unix(0, v*int64(time.Microsecond)).UTC()
	case abs >= 1e12:
		return time.Unix(0, v*int64(time.Millisecond)).UTC()
	default:
		return time.Unix(v, 0).UTC()
	}
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

type Profile struct {
	TimestampKeys  []string
	AgentKeys      []string
	ActionKeys     []string
	StatusKeys     []string
	MessageKeys    []string
	Strict         bool
	ReplaceDefault bool
}

// DefaultProfile returns the built-in parser profile.
func DefaultProfile() Profile {
	return defaultProfile()
}

// LoadEvents exposes the loader for testing and advanced workflows.
func LoadEvents(path string, profile Profile) ([]model.Event, error) {
	return loadEvents(path, profile)
}

// ParseOne exposes the single-record parser.
func ParseOne(line []byte, profile Profile) (model.Event, error) {
	return parseOne(line, profile)
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
