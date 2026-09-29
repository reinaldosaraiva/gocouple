package config

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Config holds every analysis and diagnostic threshold.
type Config struct {
	DistanceThreshold  float64
	MinCaForPain       int
	GodCeThreshold     int
	HotspotCaThreshold int
	SDPTolerance       float64
	ExportedOnly       bool
	IncludeExternal    bool
	Exclude            []string
	CompositionRoots   []string
	Check              Check
}

// Check holds the thresholds enforced by the check command.
type Check struct {
	MaxDistance     float64
	MaxPainPackages int
	FailOnCycles    bool
}

// Default returns the documented default configuration.
func Default() Config {
	return Config{
		DistanceThreshold:  0.5,
		MinCaForPain:       3,
		GodCeThreshold:     8,
		HotspotCaThreshold: 5,
		SDPTolerance:       0,
		CompositionRoots:   []string{"**/cmd/**", "**/internal/wire/**"},
		Check:              Check{MaxDistance: 0.7, MaxPainPackages: 0, FailOnCycles: true},
	}
}

// IsExcluded reports whether pkgPath matches an exclude glob.
func (c Config) IsExcluded(pkgPath string) bool {
	return matchAny(c.Exclude, pkgPath)
}

// IsCompositionRoot reports whether pkgPath matches a composition root glob.
func (c Config) IsCompositionRoot(pkgPath string) bool {
	return matchAny(c.CompositionRoots, pkgPath)
}

func matchAny(globs []string, path string) bool {
	for _, g := range globs {
		if MatchGlob(g, path) {
			return true
		}
	}
	return false
}

// MatchGlob matches an import path against a glob where ** spans path
// segments and * stays within one segment.
func MatchGlob(pattern, s string) bool {
	re, err := compiledGlob(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

var globCache sync.Map

func compiledGlob(pattern string) (*regexp.Regexp, error) {
	if cached, ok := globCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(globRegexp(pattern))
	if err != nil {
		return nil, fmt.Errorf("compiling glob %q: %w", pattern, err)
	}
	actual, _ := globCache.LoadOrStore(pattern, re)
	return actual.(*regexp.Regexp), nil
}

func globRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(pattern[i:], "/**"):
			b.WriteString("(?:/.*)?")
			i += 2
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i++
		case pattern[i] == '*':
			b.WriteString("[^/]*")
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	b.WriteString("$")
	return b.String()
}
