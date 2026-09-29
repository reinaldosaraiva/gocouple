package model

import (
	"encoding/json"
	"fmt"
)

// Entry is one history point: a snapshot, or a commit that failed to load.
type Entry struct {
	Commit   Commit
	Snapshot *Snapshot
	Error    string
}

type failedEntry struct {
	Commit Commit `json:"commit"`
	Error  string `json:"error"`
}

// MarshalJSON writes the snapshot, or {commit, error} for a failed commit.
func (e Entry) MarshalJSON() ([]byte, error) {
	if e.Error != "" || e.Snapshot == nil {
		return json.Marshal(failedEntry{Commit: e.Commit, Error: e.Error})
	}
	return json.Marshal(e.Snapshot)
}

// UnmarshalJSON reads either form written by MarshalJSON.
func (e *Entry) UnmarshalJSON(data []byte) error {
	var probe struct {
		Commit *Commit `json:"commit"`
		Error  string  `json:"error"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("decoding history entry: %w", err)
	}
	*e = Entry{}
	if probe.Commit != nil {
		e.Commit = *probe.Commit
	}
	if probe.Error != "" {
		e.Error = probe.Error
		return nil
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("decoding history snapshot: %w", err)
	}
	e.Snapshot = &snap
	return nil
}

// History is the ordered series of snapshots, oldest commit first.
type History struct {
	SchemaVersion string  `json:"schema_version"`
	Module        string  `json:"module"`
	Snapshots     []Entry `json:"snapshots"`
}
