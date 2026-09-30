package model

import (
	"encoding/json"
	"math"
)

// SchemaVersion is the JSON contract version.
const SchemaVersion = "1"

// Zone classifications.
const (
	ZoneIsolated     = "isolated"
	ZoneMainSequence = "main_sequence"
	ZonePain         = "pain"
	ZoneUselessness  = "uselessness"
)

// Diagnostic severities.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Ratio is a metric value kept at full precision and rounded to four
// decimals only when marshaled.
type Ratio float64

// MarshalJSON rounds to four decimals.
func (r Ratio) MarshalJSON() ([]byte, error) {
	return json.Marshal(math.Round(float64(r)*1e4) / 1e4)
}

// Package holds loaded facts and derived coupling metrics of one Go package.
type Package struct {
	Path         string   `json:"path"`
	IsMain       bool     `json:"is_main"`
	Ca           int      `json:"ca"`
	Ce           int      `json:"ce"`
	Instability  Ratio    `json:"instability"`
	Na           int      `json:"na"`
	Nc           int      `json:"nc"`
	Abstractness Ratio    `json:"abstractness"`
	Distance     Ratio    `json:"distance"`
	Zone         string   `json:"zone"`
	Imports      []string `json:"imports"`
	ImportedBy   []string `json:"imported_by"`
	Generated    bool     `json:"generated,omitempty"`
}

// Commit identifies the analyzed revision.
type Commit struct {
	SHA     string `json:"sha"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

// Config records the effective analysis settings.
type Config struct {
	DistanceThreshold float64 `json:"distance_threshold"`
	IncludeExternal   bool    `json:"include_external"`
	IncludeTests      bool    `json:"include_tests"`
	ExportedOnly      bool    `json:"exported_only"`
}

// Diagnostic is an architectural finding with its evidence.
type Diagnostic struct {
	ID       string         `json:"id"`
	Severity string         `json:"severity"`
	Package  string         `json:"package"`
	Message  string         `json:"message"`
	Evidence map[string]any `json:"evidence"`
}

// Suppressed is a diagnostic that was found and deliberately not reported,
// with the reason it was silenced.
type Suppressed struct {
	ID      string `json:"id"`
	Package string `json:"package"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

// Summary aggregates a snapshot.
type Summary struct {
	Packages     int   `json:"packages"`
	AvgDistance  Ratio `json:"avg_distance"`
	Pain         int   `json:"pain"`
	Uselessness  int   `json:"uselessness"`
	MainSequence int   `json:"main_sequence"`
	Isolated     int   `json:"isolated"`
	Cycles       int   `json:"cycles"`
}

// InterfaceUsage records who references an exported interface and the
// concrete module types that implement it. It is analysis input for rules
// and is not part of the JSON contract.
type InterfaceUsage struct {
	Package      string
	Name         string
	Implementers []string
	IfaceRefs    []string
	ConcreteRefs []string
}

// Snapshot is the analysis result serialized as the JSON contract.
type Snapshot struct {
	SchemaVersion string       `json:"schema_version"`
	ToolVersion   string       `json:"tool_version"`
	Module        string       `json:"module"`
	Commit        *Commit      `json:"commit,omitempty"`
	Config        Config       `json:"config"`
	Packages      []Package    `json:"packages"`
	Cycles        [][]string   `json:"cycles"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Suppressed    []Suppressed `json:"suppressed,omitempty"`
	Warnings      []string     `json:"warnings,omitempty"`
	Summary       Summary      `json:"summary"`

	Interfaces []InterfaceUsage `json:"-"`
}
