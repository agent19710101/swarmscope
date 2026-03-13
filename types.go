package main

import "time"

// Event is the normalized cross-agent record consumed by all commands.
type Event struct {
	Time    time.Time
	Agent   string
	Action  string
	Status  string
	Message string
	Source  string
}

// parserProfile maps input fields to canonical event fields.
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
