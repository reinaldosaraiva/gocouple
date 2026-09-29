package graph

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCycles(t *testing.T) {
	tests := []struct {
		name  string
		nodes []string
		edges map[string][]string
		want  [][]string
	}{
		{name: "empty"},
		{name: "single node", nodes: []string{"a"}},
		{name: "self loop ignored", nodes: []string{"a"}, edges: map[string][]string{"a": {"a"}}},
		{
			name:  "chain has no cycle",
			nodes: []string{"a", "b", "c"},
			edges: map[string][]string{"a": {"b"}, "b": {"c"}},
		},
		{
			name:  "two node cycle",
			nodes: []string{"a", "b"},
			edges: map[string][]string{"a": {"b"}, "b": {"a"}},
			want:  [][]string{{"a", "b"}},
		},
		{
			name:  "two disjoint cycles ordered by first member",
			nodes: []string{"a", "b", "c", "d"},
			edges: map[string][]string{"d": {"c"}, "c": {"d"}, "a": {"b"}, "b": {"a"}},
			want:  [][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			name:  "nested cycles form one component",
			nodes: []string{"a", "b", "c"},
			edges: map[string][]string{"a": {"b"}, "b": {"a", "c"}, "c": {"a"}},
			want:  [][]string{{"a", "b", "c"}},
		},
		{
			name:  "unknown target ignored",
			nodes: []string{"a"},
			edges: map[string][]string{"a": {"x"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.nodes, tt.edges).Cycles()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("cycles mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLongChainDoesNotOverflow(t *testing.T) {
	const size = 10000
	nodes := make([]string, size)
	edges := make(map[string][]string, size)
	for i := range nodes {
		nodes[i] = fmt.Sprintf("n%05d", i)
	}
	for i := 0; i < size-1; i++ {
		edges[nodes[i]] = []string{nodes[i+1]}
	}
	if got := New(nodes, edges).Cycles(); len(got) != 0 {
		t.Fatalf("chain reported cycles: %v", got[:1])
	}
	edges[nodes[size-1]] = []string{nodes[0]}
	got := New(nodes, edges).Cycles()
	if len(got) != 1 || len(got[0]) != size {
		t.Fatalf("closed chain: want one component of %d nodes, got %d components", size, len(got))
	}
}

func TestAdjacencyDeterministic(t *testing.T) {
	g := New([]string{"c", "a", "b", "a"}, map[string][]string{"a": {"c", "b", "b"}, "b": {"c"}})
	if diff := cmp.Diff([]string{"a", "b", "c"}, g.Nodes()); diff != "" {
		t.Errorf("nodes (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"b", "c"}, g.Imports("a")); diff != "" {
		t.Errorf("imports (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"a", "b"}, g.ImportedBy("c")); diff != "" {
		t.Errorf("imported by (-want +got):\n%s", diff)
	}
}
