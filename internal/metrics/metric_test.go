package metrics

import (
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

func TestInstability(t *testing.T) {
	tests := []struct {
		name   string
		ca, ce int
		want   float64
	}{
		{"isolated", 0, 0, 0},
		{"only afferent", 3, 0, 0},
		{"only efferent", 0, 2, 1},
		{"balanced", 2, 2, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Instability(tt.ca, tt.ce); got != tt.want {
				t.Errorf("Instability(%d,%d) = %v, want %v", tt.ca, tt.ce, got, tt.want)
			}
		})
	}
}

func TestAbstractness(t *testing.T) {
	if got := Abstractness(0, 0); got != 0 {
		t.Errorf("Nc==0: got %v, want 0", got)
	}
	if got := Abstractness(1, 4); got != 0.25 {
		t.Errorf("1/4: got %v", got)
	}
}

func TestDistance(t *testing.T) {
	tests := []struct{ a, i, want float64 }{
		{1, 1, 1},
		{0, 0, 1},
		{0, 1, 0},
		{1, 0, 0},
		{0.5, 0.5, 0},
	}
	for _, tt := range tests {
		if got := Distance(tt.a, tt.i); got != tt.want {
			t.Errorf("Distance(%v,%v) = %v, want %v", tt.a, tt.i, got, tt.want)
		}
	}
}

func TestZone(t *testing.T) {
	tests := []struct {
		name      string
		ca, ce    int
		a, i, thr float64
		want      string
	}{
		{"isolated wins over everything", 0, 0, 1, 0, 0.5, model.ZoneIsolated},
		{"on the line", 1, 1, 0.5, 0.5, 0.5, model.ZoneMainSequence},
		{"exactly at threshold", 1, 1, 0, 0.5, 0.5, model.ZoneMainSequence},
		{"just above threshold pain", 5, 0, 0.4, 0, 0.5, model.ZonePain},
		{"stable concrete", 6, 0, 0, 0, 0.5, model.ZonePain},
		{"abstract unstable", 0, 3, 1, 1, 0.5, model.ZoneUselessness},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Zone(tt.ca, tt.ce, tt.a, tt.i, tt.thr); got != tt.want {
				t.Errorf("Zone = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnalyzeChain(t *testing.T) {
	pkgs := []model.Package{
		{Path: "m/c", Na: 1, Nc: 3},
		{Path: "m/a", Imports: []string{"m/b"}, Nc: 1},
		{Path: "m/b", Imports: []string{"m/c", "ext/x", "m/c"}, Nc: 1},
	}
	res := Analyze(pkgs, 0.5)

	type row struct {
		Path       string
		Ca, Ce     int
		I, A, D    float64
		Zone       string
		ImportedBy []string
	}
	var got []row
	for _, p := range res.Packages {
		got = append(got, row{p.Path, p.Ca, p.Ce, float64(p.Instability), float64(p.Abstractness), float64(p.Distance), p.Zone, p.ImportedBy})
	}
	third := 1.0 / 3.0
	want := []row{
		{"m/a", 0, 1, 1, 0, 0, model.ZoneMainSequence, []string{}},
		{"m/b", 1, 2, 2.0 / 3.0, 0, 1.0 / 3.0, model.ZoneMainSequence, []string{"m/a"}},
		{"m/c", 1, 0, 0, third, 1 - third, model.ZonePain, []string{"m/b"}},
	}
	if diff := cmp.Diff(want, got, cmp.Comparer(func(x, y float64) bool { return math.Abs(x-y) < 1e-12 })); diff != "" {
		t.Errorf("rows (-want +got):\n%s", diff)
	}
	if len(res.Cycles) != 0 {
		t.Errorf("unexpected cycles: %v", res.Cycles)
	}
	if res.Summary.Packages != 3 || res.Summary.Pain != 1 || res.Summary.MainSequence != 2 {
		t.Errorf("summary = %+v", res.Summary)
	}
}

func TestAnalyzeCycleAndEmpty(t *testing.T) {
	res := Analyze([]model.Package{
		{Path: "m/a", Imports: []string{"m/b"}},
		{Path: "m/b", Imports: []string{"m/a"}},
	}, 0.5)
	if diff := cmp.Diff([][]string{{"m/a", "m/b"}}, res.Cycles); diff != "" {
		t.Errorf("cycles (-want +got):\n%s", diff)
	}
	if res.Summary.Cycles != 1 {
		t.Errorf("summary cycles = %d", res.Summary.Cycles)
	}
	empty := Analyze(nil, 0.5)
	if empty.Summary.Packages != 0 || empty.Summary.AvgDistance != 0 || empty.Cycles == nil {
		t.Errorf("empty result = %+v", empty)
	}
}

func TestRatioRoundsOnlyWhenMarshaled(t *testing.T) {
	r := model.Ratio(2.0 / 3.0)
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "0.6667" {
		t.Errorf("marshal = %s", b)
	}
	if float64(r) != 2.0/3.0 {
		t.Error("full precision lost")
	}
}

func TestAnalyzeIgnoresDuplicatePaths(t *testing.T) {
	res := Analyze([]model.Package{{Path: "m/a"}, {Path: "m/a"}}, 0.5)
	if res.Summary.Packages != 1 || len(res.Packages) != 1 {
		t.Errorf("packages = %d, summary = %d, want 1", len(res.Packages), res.Summary.Packages)
	}
}

func FuzzZone(f *testing.F) {
	f.Add(0, 0, 0.0, 0.0, 0.5)
	f.Add(3, 0, 0.0, 0.0, 0.5)
	f.Add(0, 2, 1.0, 1.0, 0.5)
	f.Add(1, 1, 0.0, 0.5, 0.5)
	f.Fuzz(func(t *testing.T, ca, ce int, a, i, thr float64) {
		if ca < 0 || ce < 0 || math.IsNaN(a+i+thr) || math.IsInf(a+i+thr, 0) {
			t.Skip()
		}
		zone := Zone(ca, ce, a, i, thr)
		switch zone {
		case model.ZoneIsolated:
			if ca+ce != 0 {
				t.Fatalf("isolated with ca=%d ce=%d", ca, ce)
			}
		case model.ZoneMainSequence:
			if d := Distance(a, i); d > thr {
				t.Fatalf("main sequence with D=%v > %v", d, thr)
			}
		case model.ZonePain, model.ZoneUselessness:
			if Distance(a, i) <= thr {
				t.Fatalf("%s with D=%v <= %v", zone, Distance(a, i), thr)
			}
			if (zone == model.ZonePain) != (a+i < 1) {
				t.Fatalf("%s with A+I=%v", zone, a+i)
			}
		default:
			t.Fatalf("unknown zone %q", zone)
		}
		if ca+ce == 0 && zone != model.ZoneIsolated {
			t.Fatalf("ca+ce == 0 must be isolated, got %q", zone)
		}
	})
}

func BenchmarkAnalyze1000Packages(b *testing.B) {
	pkgs := make([]model.Package, 1000)
	for i := range pkgs {
		pkgs[i] = model.Package{Path: fmt.Sprintf("m/p%04d", i), Na: i % 3, Nc: 3}
		for j := 1; j <= 5 && i+j < len(pkgs); j++ {
			pkgs[i].Imports = append(pkgs[i].Imports, fmt.Sprintf("m/p%04d", i+j))
		}
	}
	for b.Loop() {
		Analyze(pkgs, 0.5)
	}
}
