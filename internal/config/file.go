package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the configuration file discovered in the module root.
const FileName = ".gocouple.yaml"

type fileConfig struct {
	DistanceThreshold  *float64     `yaml:"distance_threshold"`
	MinCaForPain       *int         `yaml:"min_ca_for_pain"`
	GodCeThreshold     *int         `yaml:"god_ce_threshold"`
	HotspotCaThreshold *int         `yaml:"hotspot_ca_threshold"`
	SDPTolerance       *float64     `yaml:"sdp_tolerance"`
	ExportedOnly       *bool        `yaml:"exported_only"`
	IncludeExternal    *bool        `yaml:"include_external"`
	Exclude            *[]string    `yaml:"exclude"`
	CompositionRoots   *[]string    `yaml:"composition_roots"`
	Ignore             []fileIgnore `yaml:"ignore"`
	Check              *fileCheck   `yaml:"check"`
}

type fileIgnore struct {
	Rule    string `yaml:"rule"`
	Package string `yaml:"package"`
	Reason  string `yaml:"reason"`
}

type fileCheck struct {
	MaxDistance     *float64 `yaml:"max_distance"`
	MaxPainPackages *int     `yaml:"max_pain_packages"`
	FailOnCycles    *bool    `yaml:"fail_on_cycles"`
}

// Parse decodes YAML over the defaults. Unknown keys and out-of-range values
// are errors that name the key.
func Parse(data []byte) (Config, error) {
	cfg := Default()
	var f fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	set(&cfg.DistanceThreshold, f.DistanceThreshold)
	set(&cfg.MinCaForPain, f.MinCaForPain)
	set(&cfg.GodCeThreshold, f.GodCeThreshold)
	set(&cfg.HotspotCaThreshold, f.HotspotCaThreshold)
	set(&cfg.SDPTolerance, f.SDPTolerance)
	set(&cfg.ExportedOnly, f.ExportedOnly)
	set(&cfg.IncludeExternal, f.IncludeExternal)
	set(&cfg.Exclude, f.Exclude)
	set(&cfg.CompositionRoots, f.CompositionRoots)
	for _, r := range f.Ignore {
		cfg.Ignore = append(cfg.Ignore, IgnoreRule(r))
	}
	if c := f.Check; c != nil {
		set(&cfg.Check.MaxDistance, c.MaxDistance)
		set(&cfg.Check.MaxPainPackages, c.MaxPainPackages)
		set(&cfg.Check.FailOnCycles, c.FailOnCycles)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func set[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

// Validate checks value ranges.
func (c Config) Validate() error {
	switch {
	case c.DistanceThreshold < 0 || c.DistanceThreshold > 1:
		return fmt.Errorf("config: distance_threshold must be in [0,1], got %v", c.DistanceThreshold)
	case c.MinCaForPain < 0:
		return fmt.Errorf("config: min_ca_for_pain must be >= 0, got %d", c.MinCaForPain)
	case c.GodCeThreshold < 0:
		return fmt.Errorf("config: god_ce_threshold must be >= 0, got %d", c.GodCeThreshold)
	case c.HotspotCaThreshold < 0:
		return fmt.Errorf("config: hotspot_ca_threshold must be >= 0, got %d", c.HotspotCaThreshold)
	case c.SDPTolerance < 0 || c.SDPTolerance > 1:
		return fmt.Errorf("config: sdp_tolerance must be in [0,1], got %v", c.SDPTolerance)
	case c.Check.MaxDistance < 0 || c.Check.MaxDistance > 1:
		return fmt.Errorf("config: check.max_distance must be in [0,1], got %v", c.Check.MaxDistance)
	case c.Check.MaxPainPackages < 0:
		return fmt.Errorf("config: check.max_pain_packages must be >= 0, got %d", c.Check.MaxPainPackages)
	}
	for i, r := range c.Ignore {
		switch {
		case !slices.Contains(KnownRules, r.Rule):
			return fmt.Errorf("config: ignore[%d].rule %q is not a known rule (%s)", i, r.Rule, strings.Join(KnownRules, ", "))
		case r.Package == "":
			return fmt.Errorf("config: ignore[%d].package must not be empty", i)
		case strings.TrimSpace(r.Reason) == "":
			return fmt.Errorf("config: ignore[%d].reason must not be empty: say why %s on %s is acceptable", i, r.Rule, r.Package)
		}
	}
	return nil
}

// Load reads a configuration file. An empty file yields the defaults.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Discover loads the explicit path when given, otherwise FileName inside dir
// when it exists, otherwise the defaults.
func Discover(dir, explicit string) (Config, error) {
	if explicit != "" {
		return Load(explicit)
	}
	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("checking %s: %w", path, err)
	}
	return Load(path)
}
