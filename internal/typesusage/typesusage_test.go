package typesusage

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

type memImporter struct {
	std  types.Importer
	pkgs map[string]*types.Package
}

func (m memImporter) Import(path string) (*types.Package, error) {
	if p, ok := m.pkgs[path]; ok {
		return p, nil
	}
	return m.std.Import(path)
}

// check type-checks the given packages in order; a package named main is
// flagged IsMain.
func check(t *testing.T, order []string, src map[string]string) []Package {
	t.Helper()
	fset := token.NewFileSet()
	imp := memImporter{std: importer.ForCompiler(fset, "source", nil), pkgs: map[string]*types.Package{}}
	var out []Package
	for _, path := range order {
		f, err := parser.ParseFile(fset, path+".go", src[path], 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
		pkg, err := (&types.Config{Importer: imp}).Check(path, fset, []*ast.File{f}, info)
		if err != nil {
			t.Fatalf("check %s: %v", path, err)
		}
		imp.pkgs[path] = pkg
		out = append(out, Package{Path: path, IsMain: pkg.Name() == "main", Types: pkg, Info: info})
	}
	return out
}

func usage(t *testing.T, order []string, src map[string]string, roots ...string) map[string]model.InterfaceUsage {
	t.Helper()
	isComp := func(p string) bool {
		for _, r := range roots {
			if strings.HasPrefix(p, r) {
				return true
			}
		}
		return false
	}
	got := map[string]model.InterfaceUsage{}
	for _, u := range Analyze(check(t, order, src), isComp) {
		got[u.Package+"."+u.Name] = u
	}
	return got
}

const (
	contracts = "package contracts\n\ntype Doer interface{ Do() }\n"
	impl      = "package impl\n\ntype Real struct{}\n\nfunc (*Real) Do() {}\n\nfunc NewReal() *Real { return &Real{} }\n"
)

func TestInterfaceWithConsumersOnly(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl,
		"m/user":      "package user\n\nimport \"m/contracts\"\n\ntype U struct{ D contracts.Doer }\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl", "m/user"}, src)["m/contracts.Doer"]
	want := model.InterfaceUsage{
		Package: "m/contracts", Name: "Doer",
		Implementers: []string{"m/impl.Real"}, IfaceRefs: []string{"m/user"}, ConcreteRefs: []string{},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestConcreteOnlyUse(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl,
		"m/user":      "package user\n\nimport \"m/impl\"\n\nvar _ = &impl.Real{}\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl", "m/user"}, src)["m/contracts.Doer"]
	if len(got.IfaceRefs) != 0 || !cmp.Equal(got.ConcreteRefs, []string{"m/user"}) {
		t.Errorf("usage = %+v", got)
	}
}

func TestConstructorCallCountsAsConcrete(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl,
		"m/user":      "package user\n\nimport \"m/impl\"\n\nvar r = impl.NewReal()\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl", "m/user"}, src)["m/contracts.Doer"]
	if !cmp.Equal(got.ConcreteRefs, []string{"m/user"}) {
		t.Errorf("concrete refs = %v", got.ConcreteRefs)
	}
}

func TestBothInterfaceAndConcrete(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl,
		"m/a":         "package a\n\nimport \"m/contracts\"\n\nvar _ contracts.Doer\n",
		"m/b":         "package b\n\nimport \"m/impl\"\n\nvar _ impl.Real\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl", "m/a", "m/b"}, src)["m/contracts.Doer"]
	if !cmp.Equal(got.IfaceRefs, []string{"m/a"}) || !cmp.Equal(got.ConcreteRefs, []string{"m/b"}) {
		t.Errorf("usage = %+v", got)
	}
}

func TestCompositionRootAndMainDoNotCountAsConcrete(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl,
		"m/cmd/app":   "package app\n\nimport \"m/impl\"\n\nvar _ = &impl.Real{}\n",
		"m/entry":     "package main\n\nimport \"m/impl\"\n\nvar _ = &impl.Real{}\n\nfunc main() {}\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl", "m/cmd/app", "m/entry"}, src, "m/cmd/")["m/contracts.Doer"]
	if len(got.ConcreteRefs) != 0 {
		t.Errorf("concrete refs = %v", got.ConcreteRefs)
	}
}

func TestImplementerOwnPackageIgnored(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl + "\nvar _ = &Real{}\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl"}, src)["m/contracts.Doer"]
	if len(got.ConcreteRefs) != 0 {
		t.Errorf("concrete refs = %v", got.ConcreteRefs)
	}
}

func TestSkips(t *testing.T) {
	src := map[string]string{
		"m/p": `package p

type unexported interface{ Do() }
type Empty interface{}
type Constraint interface{ ~int }
type Generic[T any] interface{ Get() T }
type Alias = Real
type Real struct{}

func (Real) Do() {}
`,
	}
	if got := usage(t, []string{"m/p"}, src); len(got) != 0 {
		t.Errorf("expected no interfaces, got %v", got)
	}
}

func TestValueReceiverAndEmbeddedInterface(t *testing.T) {
	src := map[string]string{
		"m/p": `package p

type Reader interface{ Read() string }
type ReadCloser interface {
	Reader
	Close()
}
type File struct{}

func (File) Read() string { return "" }
func (*File) Close()      {}
`,
	}
	got := usage(t, []string{"m/p"}, src)
	if diff := cmp.Diff([]string{"m/p.File"}, got["m/p.ReadCloser"].Implementers); diff != "" {
		t.Errorf("ReadCloser implementers (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"m/p.File"}, got["m/p.Reader"].Implementers); diff != "" {
		t.Errorf("Reader implementers (-want +got):\n%s", diff)
	}
}

func TestWrappedConstructorResultAndAlias(t *testing.T) {
	src := map[string]string{
		"m/contracts": contracts,
		"m/impl":      impl + "\nfunc NewMany() []*Real { return nil }\n",
		"m/a":         "package a\n\nimport \"m/impl\"\n\nvar _ = impl.NewMany()\n",
		"m/alias":     "package alias\n\nimport \"m/impl\"\n\ntype R = impl.Real\n\nvar _ R\n",
	}
	got := usage(t, []string{"m/contracts", "m/impl", "m/a", "m/alias"}, src)["m/contracts.Doer"]
	if !cmp.Equal(got.ConcreteRefs, []string{"m/a", "m/alias"}) {
		t.Errorf("concrete refs = %v", got.ConcreteRefs)
	}
}
