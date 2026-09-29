package graph

import (
	"slices"
	"sort"
)

// Graph is an immutable directed dependency graph over a fixed node set.
// Edges to unknown nodes and self-edges are dropped: a package importing
// itself is not a cycle of two or more packages and cannot compile in Go.
type Graph struct {
	nodes []string
	out   map[string][]string
	in    map[string][]string
}

// New builds a graph over nodes with the given adjacency lists.
func New(nodes []string, edges map[string][]string) *Graph {
	g := &Graph{
		nodes: slices.Clone(nodes),
		out:   make(map[string][]string, len(nodes)),
		in:    make(map[string][]string, len(nodes)),
	}
	sort.Strings(g.nodes)
	g.nodes = slices.Compact(g.nodes)
	known := make(map[string]bool, len(g.nodes))
	for _, n := range g.nodes {
		known[n] = true
		g.out[n] = []string{}
		g.in[n] = []string{}
	}
	for _, from := range g.nodes {
		for _, to := range edges[from] {
			if !known[to] || to == from {
				continue
			}
			g.out[from] = append(g.out[from], to)
		}
		sort.Strings(g.out[from])
		g.out[from] = slices.Compact(g.out[from])
		for _, to := range g.out[from] {
			g.in[to] = append(g.in[to], from)
		}
	}
	return g
}

// Nodes returns the sorted node names.
func (g *Graph) Nodes() []string { return slices.Clone(g.nodes) }

// Imports returns the sorted direct dependencies of n.
func (g *Graph) Imports(n string) []string { return slices.Clone(g.out[n]) }

// ImportedBy returns the sorted direct dependents of n.
func (g *Graph) ImportedBy(n string) []string { return slices.Clone(g.in[n]) }

// Cycles returns the strongly connected components with two or more nodes
// using an iterative Tarjan algorithm, so deep chains do not grow the call
// stack. Members are sorted and the components are ordered by first member.
func (g *Graph) Cycles() [][]string {
	n := len(g.nodes)
	id := make(map[string]int, n)
	for i, name := range g.nodes {
		id[name] = i
	}
	adj := make([][]int, n)
	for i, name := range g.nodes {
		for _, to := range g.out[name] {
			adj[i] = append(adj[i], id[to])
		}
	}

	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	type frame struct{ v, next int }
	var (
		counter int
		stack   []int
		calls   []frame
		result  [][]string
	)
	visit := func(v int) {
		index[v], low[v] = counter, counter
		counter++
		stack = append(stack, v)
		onStack[v] = true
		calls = append(calls, frame{v: v})
	}

	for root := range n {
		if index[root] != -1 {
			continue
		}
		visit(root)
		for len(calls) > 0 {
			top := len(calls) - 1
			v := calls[top].v
			if calls[top].next < len(adj[v]) {
				w := adj[v][calls[top].next]
				calls[top].next++
				switch {
				case index[w] == -1:
					visit(w)
				case onStack[w]:
					low[v] = min(low[v], index[w])
				}
				continue
			}
			calls = calls[:top]
			if low[v] == index[v] {
				var comp []string
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					comp = append(comp, g.nodes[w])
					if w == v {
						break
					}
				}
				if len(comp) >= 2 {
					sort.Strings(comp)
					result = append(result, comp)
				}
			}
			if len(calls) > 0 {
				parent := calls[len(calls)-1].v
				low[parent] = min(low[parent], low[v])
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i][0] < result[j][0] })
	return result
}
