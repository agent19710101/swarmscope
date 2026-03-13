package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Filter utilities implement cross-command event slicing and windowing behavior.
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
