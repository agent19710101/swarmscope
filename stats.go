package main

import (
	"fmt"
	"sort"
	"time"
)

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
