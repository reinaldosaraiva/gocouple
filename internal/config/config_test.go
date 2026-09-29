package config

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, path string
		want          bool
	}{
		{"**/cmd/**", "example.com/m/cmd/orders", true},
		{"**/cmd/**", "example.com/m/cmd", true},
		{"**/cmd/**", "cmd/x", true},
		{"**/cmd/**", "example.com/m/internal/cmdx", false},
		{"**/internal/wire/**", "example.com/m/internal/wire", true},
		{"**/mocks/**", "example.com/m/mocks/a/b", true},
		{"example.com/*/x", "example.com/m/x", true},
		{"example.com/*/x", "example.com/m/n/x", false},
		{"a.b", "aXb", false},
	}
	for _, tt := range tests {
		if got := MatchGlob(tt.pattern, tt.path); got != tt.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestDefaultCompositionRoots(t *testing.T) {
	c := Default()
	if !c.IsCompositionRoot("example.com/m/cmd/app") || c.IsCompositionRoot("example.com/m/internal/order") {
		t.Error("default composition roots misclassify")
	}
}

func TestMatchGlobNonASCII(t *testing.T) {
	if !MatchGlob("**/café/**", "example.com/café/x") {
		t.Error("non-ASCII glob must match")
	}
}
