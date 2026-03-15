package model

import "time"

// Event is the normalized record produced by the ingest layer.
type Event struct {
	Time    time.Time
	Agent   string
	Action  string
	Status  string
	Message string
	Source  string
}
