package loader

import (
	"context"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/typesusage"
	"golang.org/x/tools/go/packages"
)

// Options selects what to load and which dependencies count.
type Options struct {
	Dir             string
	Patterns        []string
	IncludeExternal bool
	IncludeTests    bool
	ExportedOnly    bool
}

// Result is the module path and its loaded packages sorted by path.
type Result struct {
	Module   string
	Packages []model.Package
	Typed    []typesusage.Package
}

const mode = packages.NeedName | packages.NeedFiles | packages.NeedImports |
	packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule |
	packages.NeedDeps

// Load reads the packages matching opts.Patterns and converts them to the
// neutral model. Derived metrics are left zero.
func Load(ctx context.Context, opts Options) (Result, error) {
	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	cfg := &packages.Config{Context: ctx, Dir: opts.Dir, Mode: mode, Tests: opts.IncludeTests}
	loaded, err := packages.Load(cfg, patterns...)
	if err != nil {
		return Result{}, fmt.Errorf("loading %v in %q: %w", patterns, opts.Dir, err)
	}
	if err := loadErrors(loaded); err != nil {
		return Result{}, err
	}

	module := mainModule(loaded)
	if module == "" {
		return Result{}, fmt.Errorf("loading %v in %q: no packages matched", patterns, opts.Dir)
	}

	byPath := make(map[string]*packages.Package, len(loaded))
	for _, p := range loaded {
		if skip(p) {
			continue
		}
		if prev, ok := byPath[p.PkgPath]; ok && !isTestVariant(p) && isTestVariant(prev) {
			continue
		}
		byPath[p.PkgPath] = p
	}

	out := make([]model.Package, 0, len(byPath))
	typed := make([]typesusage.Package, 0, len(byPath))
	for path, p := range byPath {
		out = append(out, convert(path, p, module, opts))
		typed = append(typed, typesusage.Package{Path: path, IsMain: p.Name == "main", Types: p.Types, Info: p.TypesInfo})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	sort.Slice(typed, func(i, j int) bool { return typed[i].Path < typed[j].Path })
	return Result{Module: module, Packages: out, Typed: typed}, nil
}

func loadErrors(pkgs []*packages.Package) error {
	var msgs []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			msgs = append(msgs, fmt.Sprintf("%s: %s", p.PkgPath, e.Msg))
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	sort.Strings(msgs)
	return fmt.Errorf("loading packages: %s", strings.Join(msgs, "; "))
}

func mainModule(pkgs []*packages.Package) string {
	var found []string
	for _, p := range pkgs {
		if p.Module != nil && p.Module.Main {
			found = append(found, p.Module.Path)
		}
	}
	if len(found) == 0 {
		return ""
	}
	sort.Strings(found)
	return found[0]
}

func skip(p *packages.Package) bool {
	return len(p.GoFiles) == 0 || strings.HasSuffix(p.PkgPath, ".test")
}

func isTestVariant(p *packages.Package) bool {
	return strings.Contains(p.ID, " [")
}

func convert(path string, p *packages.Package, module string, opts Options) model.Package {
	out := model.Package{Path: path, IsMain: p.Name == "main", Imports: []string{}}
	for imp := range p.Imports {
		if imp == path {
			continue
		}
		switch classify(imp, module) {
		case internalDep:
			out.Imports = append(out.Imports, imp)
		case externalDep:
			if opts.IncludeExternal {
				out.Imports = append(out.Imports, imp)
			}
		}
	}
	sort.Strings(out.Imports)
	out.Nc, out.Na = countTypes(p.Types, opts.ExportedOnly)
	return out
}

type depKind int

const (
	internalDep depKind = iota
	externalDep
	stdlibDep
)

func classify(path, module string) depKind {
	if path == module || strings.HasPrefix(path, module+"/") {
		return internalDep
	}
	first, _, _ := strings.Cut(path, "/")
	if !strings.Contains(first, ".") {
		return stdlibDep
	}
	return externalDep
}

// countTypes returns the number of declared named types (aliases excluded)
// and how many of them are method-set interfaces; constraint-only
// interfaces count as types but not as abstractions.
func countTypes(pkg *types.Package, exportedOnly bool) (nc, na int) {
	if pkg == nil {
		return 0, 0
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if exportedOnly && !tn.Exported() {
			continue
		}
		nc++
		if iface, ok := tn.Type().Underlying().(*types.Interface); ok && iface.IsMethodSet() {
			na++
		}
	}
	return nc, na
}
