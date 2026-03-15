package feed

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent19710101/swarmscope/internal/model"
)

// NormalizeTimeWindowArgs converts CLI time window flags into explicit timestamps.
func NormalizeTimeWindowArgs(sinceRaw, untilRaw, lastRaw string, now time.Time) (string, string, error) {
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

// ApplyTimeWindow filters events based on the provided RFC3339 window.
func ApplyTimeWindow(events []model.Event, sinceRaw, untilRaw string) ([]model.Event, error) {
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

// ApplyAgentFilter filters events by a comma-separated list of agents.
func ApplyAgentFilter(events []model.Event, raw string) []model.Event {
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

// ApplySourceFilter filters events by source path or basename.
func ApplySourceFilter(events []model.Event, raw string) []model.Event {
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

// ApplyContainsFilter filters events whose message contains the given substring.
func ApplyContainsFilter(events []model.Event, raw string) []model.Event {
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

// ApplyLimit trims the event slice according to --limit and --tail flags.
func ApplyLimit(events []model.Event, limit int, tail bool) []model.Event {
	if limit <= 0 || limit >= len(events) {
		return events
	}
	if tail {
		return events[len(events)-limit:]
	}
	return events[:limit]
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
