package diagnose

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type dependencyCycle struct{}

func (dependencyCycle) Check(_ context.Context, snap *model.Snapshot, _ config.Config) []model.Diagnostic {
	imports := make(map[string][]string, len(snap.Packages))
	for _, p := range snap.Packages {
		imports[p.Path] = p.Imports
	}
	var out []model.Diagnostic
	for _, members := range snap.Cycles {
		if len(members) < 2 {
			continue
		}
		path := cyclePath(members, imports)
		out = append(out, model.Diagnostic{
			ID:       "dependency-cycle",
			Severity: model.SeverityError,
			Package:  members[0],
			Message:  fmt.Sprintf("import cycle: %s -> %s", strings.Join(path, " -> "), path[0]),
			Evidence: map[string]any{"cycle": path, "members": members},
		})
	}
	return out
}

// cyclePath returns the shortest cycle through the smallest member, ordered
// from that member; members is sorted and connected.
func cyclePath(members []string, imports map[string][]string) []string {
	start := members[0]
	inSCC := make(map[string]bool, len(members))
	for _, m := range members {
		inSCC[m] = true
	}
	prev := map[string]string{}
	queue := []string{start}
	seen := map[string]bool{start: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		next := slices.Clone(imports[cur])
		slices.Sort(next)
		for _, n := range next {
			if !inSCC[n] {
				continue
			}
			if n == start {
				path := []string{cur}
				for path[len(path)-1] != start {
					path = append(path, prev[path[len(path)-1]])
				}
				slices.Reverse(path)
				return path
			}
			if !seen[n] {
				seen[n] = true
				prev[n] = cur
				queue = append(queue, n)
			}
		}
	}
	return members
}
