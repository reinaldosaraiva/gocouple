package loader

import (
	"strings"

	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reinaldosaraiva/gocouple/internal/model"
	"github.com/reinaldosaraiva/gocouple/internal/typesusage"

	"github.com/google/go-cmp/cmp"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadSimple(t *testing.T) {
	res, err := Load(t.Context(), Options{Dir: fixture(t, "simple")})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Module != "example.com/simple" {
		t.Errorf("module = %q", res.Module)
	}
	gotDir, err := os.Stat(res.ModuleDir)
	if err != nil {
		t.Fatalf("module dir %q: %v", res.ModuleDir, err)
	}
	wantDir, err := os.Stat(fixture(t, "simple"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotDir, wantDir) {
		t.Errorf("module dir = %q, want the simple fixture", res.ModuleDir)
	}
	type row struct {
		Path    string
		Imports []string
		Nc, Na  int
	}
	var got []row
	for _, p := range res.Packages {
		got = append(got, row{p.Path, p.Imports, p.Nc, p.Na})
	}
	want := []row{
		{"example.com/simple/a", []string{"example.com/simple/b"}, 1, 0},
		{"example.com/simple/b", []string{"example.com/simple/c"}, 1, 0},
		{"example.com/simple/c", []string{}, 3, 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("packages (-want +got):\n%s", diff)
	}
}

func TestLoadExportedOnly(t *testing.T) {
	res, err := Load(t.Context(), Options{Dir: fixture(t, "simple"), ExportedOnly: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, p := range res.Packages {
		if p.Path == "example.com/simple/b" && p.Nc != 1 {
			t.Errorf("b Nc = %d, want 1", p.Nc)
		}
	}
}

func TestLoadNoPackages(t *testing.T) {
	_, err := Load(context.Background(), Options{Dir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for a directory without a module")
	}
}

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadIncludeTests(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":       "module example.com/t\n\ngo 1.26\n",
		"p/p.go":       "package p\n\ntype T struct{}\n",
		"p/p_test.go":  "package p_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/t/p\"\n)\n\nfunc TestX(t *testing.T) { _ = p.T{} }\n",
		"main/main.go": "package main\n\nimport _ \"example.com/t/p\"\n\nfunc main() {}\n",
	})
	without, err := Load(t.Context(), Options{Dir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(without.Packages); got != 2 {
		t.Errorf("without tests: %d packages, want 2", got)
	}
	with, err := Load(t.Context(), Options{Dir: dir, IncludeTests: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var paths []string
	isMain := map[string]bool{}
	for _, p := range with.Packages {
		paths = append(paths, p.Path)
		isMain[p.Path] = p.IsMain
	}
	want := []string{"example.com/t/main", "example.com/t/p", "example.com/t/p_test"}
	if diff := cmp.Diff(want, paths); diff != "" {
		t.Errorf("paths (-want +got):\n%s", diff)
	}
	if !isMain["example.com/t/main"] || isMain["example.com/t/p"] {
		t.Errorf("is_main flags = %v", isMain)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		path string
		want depKind
	}{
		{"example.com/m", internalDep},
		{"example.com/m/x", internalDep},
		{"example.com/mx", externalDep},
		{"fmt", stdlibDep},
		{"golang.org/x/tools", externalDep},
		{"net/http", stdlibDep},
	}
	for _, tt := range tests {
		if got := classify(tt.path, "example.com/m"); got != tt.want {
			t.Errorf("classify(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestLoadInPackageTestsReplacePlainPackage(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":      "module example.com/t\n\ngo 1.26\n",
		"p/p.go":      "package p\n\ntype T struct{}\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\ntype helper struct{}\n\nfunc TestX(t *testing.T) { _ = helper{} }\n",
	})
	plain, err := Load(t.Context(), Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	with, err := Load(t.Context(), Options{Dir: dir, IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Packages) != 1 || plain.Packages[0].Nc != 1 {
		t.Errorf("without tests: %+v", plain.Packages)
	}
	if len(with.Packages) != 1 || with.Packages[0].Nc != 2 {
		t.Errorf("with tests the test-augmented variant must replace the plain package once: %+v", with.Packages)
	}
}

func TestLoadIncludeExternal(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	for name, body := range map[string]string{
		"ext/go.mod": "module example.org/ext\n\ngo 1.26\n",
		"ext/ext.go": "package ext\n\nfunc Name() string { return \"x\" }\n",
		"app/go.mod": "module example.com/app\n\ngo 1.26\n\nrequire example.org/ext v0.0.0\n\nreplace example.org/ext => ../ext\n",
		"app/a/a.go": "package a\n\nimport (\n\t\"fmt\"\n\n\t\"example.org/ext\"\n)\n\nvar _ = fmt.Sprint(ext.Name())\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	without, err := Load(t.Context(), Options{Dir: app})
	if err != nil {
		t.Fatal(err)
	}
	with, err := Load(t.Context(), Options{Dir: app, IncludeExternal: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := without.Packages[0].Imports; len(got) != 0 {
		t.Errorf("external and stdlib imports must not count by default: %v", got)
	}
	if diff := cmp.Diff([]string{"example.org/ext"}, with.Packages[0].Imports); diff != "" {
		t.Errorf("--include-external (-want +got):\n%s", diff)
	}
}

func TestLoadReportsTypeErrors(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module example.com/t\n\ngo 1.26\n",
		"p/p.go": "package p\n\nvar x int = \"not an int\"\n",
	})
	_, err := Load(t.Context(), Options{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "example.com/t/p") {
		t.Errorf("err = %v, want a load error naming the package", err)
	}
}

func TestExclude(t *testing.T) {
	res := Result{
		Module: "m",
		Packages: []model.Package{
			{Path: "m/a", Imports: []string{"m/b", "m/mocks"}},
			{Path: "m/mocks"},
		},
		Typed: []typesusage.Package{{Path: "m/a"}, {Path: "m/mocks"}},
	}
	got := Exclude(res, func(p string) bool { return p == "m/mocks" })
	if len(got.Packages) != 1 || len(got.Typed) != 1 || !cmp.Equal(got.Packages[0].Imports, []string{"m/b"}) {
		t.Errorf("Exclude = %+v", got)
	}
	if len(res.Packages[0].Imports) != 2 {
		t.Error("Exclude must not mutate its input")
	}
	empty := Exclude(res, func(string) bool { return true })
	if empty.Packages == nil || empty.Typed == nil {
		t.Error("empty results must be non-nil slices")
	}
}
