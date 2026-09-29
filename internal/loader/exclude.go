package loader

import (
	"slices"

	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/typesusage"
)

// Exclude drops the packages for which excluded returns true and removes
// them from the imports of the remaining packages, so they affect no metric.
func Exclude(res Result, excluded func(path string) bool) Result {
	out := Result{Module: res.Module}
	for _, p := range res.Packages {
		if excluded(p.Path) {
			continue
		}
		p.Imports = slices.DeleteFunc(slices.Clone(p.Imports), excluded)
		out.Packages = append(out.Packages, p)
	}
	for _, t := range res.Typed {
		if !excluded(t.Path) {
			out.Typed = append(out.Typed, t)
		}
	}
	if out.Packages == nil {
		out.Packages = []model.Package{}
	}
	if out.Typed == nil {
		out.Typed = []typesusage.Package{}
	}
	return out
}
