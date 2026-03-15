package stats

import (
	"sort"
	"time"

	"github.com/agent19710101/swarmscope/internal/ingest"
	"github.com/agent19710101/swarmscope/internal/model"
)

// TimeWindow describes the start/end timestamps that were observed.
type TimeWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration string `json:"duration"`
}

// Summary is the aggregated stats output for the stats command.
type Summary struct {
	Events  int            `json:"events"`
	Window  TimeWindow     `json:"window"`
	Agents  map[string]int `json:"agents"`
	Actions map[string]int `json:"actions"`
	Status  map[string]int `json:"status"`
}

// Output is the JSON envelope for the stats command.
type Output struct {
	Summary
	Skipped     int                      `json:"skipped,omitempty"`
	Diagnostics []ingest.LoadDiagnostics `json:"diagnostics,omitempty"`
}

// AgentSummary describes the per-agent aggregation used for the agent command.
type AgentSummary struct {
	Agent     string `json:"agent"`
	Events    int    `json:"events"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Actions   int    `json:"actions"`
	Statuses  int    `json:"statuses"`
}

// AgentOutput is the JSON envelope for the agent command.
type AgentOutput struct {
	Agents      []AgentSummary           `json:"agents"`
	Skipped     int                      `json:"skipped,omitempty"`
	Diagnostics []ingest.LoadDiagnostics `json:"diagnostics,omitempty"`
}

// BuildSummary aggregates the provided events into counts and a time window.
func BuildSummary(events []model.Event) Summary {
	agentCount := map[string]int{}
	actionCount := map[string]int{}
	statusCount := map[string]int{}

	for _, ev := range events {
		agentCount[ev.Agent]++
		actionCount[ev.Action]++
		statusCount[ev.Status]++
	}

	duration := events[len(events)-1].Time.Sub(events[0].Time)
	return Summary{
		Events: len(events),
		Window: TimeWindow{
			Start:    events[0].Time.Format(time.RFC3339),
			End:      events[len(events)-1].Time.Format(time.RFC3339),
			Duration: duration.String(),
		},
		Agents:  agentCount,
		Actions: actionCount,
		Status:  statusCount,
	}
}

// BuildAgentSummaries produces per-agent stats for the agent command.
func BuildAgentSummaries(events []model.Event) []AgentSummary {
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

	out := make([]AgentSummary, 0, len(byAgent))
	for agent, entry := range byAgent {
		out = append(out, AgentSummary{
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
