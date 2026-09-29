package typesusage

import (
	"go/types"
	"sort"

	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// Package is a type-checked module package.
type Package struct {
	Path   string
	IsMain bool
	Types  *types.Package
	Info   *types.Info

	GeneratedTypes map[string]bool
}

type key struct{ pkg, name string }

func (k key) String() string { return k.pkg + "." + k.name }

// Analyze computes, for every exported method-set interface declared in the
// given packages, which other packages reference the interface and which
// reference the concrete module types that implement it. Packages named main
// and those for which isComposition returns true never count as concrete
// references. References from the interface's own package or from the
// implementer's own package are ignored.
func Analyze(pkgs []Package, isComposition func(path string) bool) []model.InterfaceUsage {
	ifaces := exportedInterfaces(pkgs)
	implFor := implementers(pkgs, ifaces)

	ifaceRefs := make(map[key]map[string]bool, len(ifaces))
	concreteRefs := make(map[key]map[string]bool, len(ifaces))
	for k := range ifaces {
		ifaceRefs[k] = map[string]bool{}
		concreteRefs[k] = map[string]bool{}
	}
	countConcrete := func(from string, isMain bool, impl key) {
		if isMain || isComposition(from) || from == impl.pkg {
			return
		}
		for _, x := range implFor[impl] {
			if from != x.pkg {
				concreteRefs[x][from] = true
			}
		}
	}

	for _, p := range pkgs {
		if p.Info == nil {
			continue
		}
		for _, obj := range p.Info.Uses {
			switch o := obj.(type) {
			case *types.TypeName:
				if o.IsAlias() {
					named := namedOf(o.Type())
					if named == nil {
						continue
					}
					o = named.Obj()
				}
				k, ok := keyOf(o)
				if !ok {
					continue
				}
				if _, isIface := ifaces[k]; isIface && p.Path != k.pkg {
					ifaceRefs[k][p.Path] = true
				}
				countConcrete(p.Path, p.IsMain, k)
			case *types.Func:
				sig, ok := o.Type().(*types.Signature)
				if !ok || sig.Recv() != nil {
					continue
				}
				for v := range sig.Results().Variables() {
					if named := namedOf(v.Type()); named != nil {
						if k, ok := keyOf(named.Obj()); ok {
							countConcrete(p.Path, p.IsMain, k)
						}
					}
				}
			}
		}
	}

	out := make([]model.InterfaceUsage, 0, len(ifaces))
	for k := range ifaces {
		impls := make([]string, 0)
		for impl, xs := range implFor {
			for _, x := range xs {
				if x == k {
					impls = append(impls, impl.String())
				}
			}
		}
		sort.Strings(impls)
		out = append(out, model.InterfaceUsage{
			Package:      k.pkg,
			Name:         k.name,
			Implementers: impls,
			IfaceRefs:    sortedKeys(ifaceRefs[k]),
			ConcreteRefs: sortedKeys(concreteRefs[k]),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func exportedInterfaces(pkgs []Package) map[key]*types.Interface {
	out := map[key]*types.Interface{}
	for _, p := range pkgs {
		if p.Types == nil {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() || !tn.Exported() || isGeneric(tn) || p.GeneratedTypes[name] {
				continue
			}
			iface, ok := tn.Type().Underlying().(*types.Interface)
			if !ok || !iface.IsMethodSet() || iface.NumMethods() == 0 {
				continue
			}
			out[key{p.Path, name}] = iface
		}
	}
	return out
}

func implementers(pkgs []Package, ifaces map[key]*types.Interface) map[key][]key {
	out := map[key][]key{}
	for _, p := range pkgs {
		if p.Types == nil {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() || isGeneric(tn) {
				continue
			}
			if _, isIface := tn.Type().Underlying().(*types.Interface); isIface {
				continue
			}
			self := key{p.Path, name}
			for ik, iface := range ifaces {
				if types.Implements(tn.Type(), iface) || types.Implements(types.NewPointer(tn.Type()), iface) {
					out[self] = append(out[self], ik)
				}
			}
		}
	}
	return out
}

func isGeneric(tn *types.TypeName) bool {
	named, ok := tn.Type().(*types.Named)
	return ok && named.TypeParams().Len() > 0
}

func keyOf(tn *types.TypeName) (key, bool) {
	if tn.Pkg() == nil {
		return key{}, false
	}
	return key{tn.Pkg().Path(), tn.Name()}, true
}

func namedOf(t types.Type) *types.Named {
	for {
		switch u := types.Unalias(t).(type) {
		case *types.Pointer:
			t = u.Elem()
		case *types.Slice:
			t = u.Elem()
		case *types.Array:
			t = u.Elem()
		case *types.Map:
			t = u.Elem()
		case *types.Chan:
			t = u.Elem()
		case *types.Named:
			return u
		default:
			return nil
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
