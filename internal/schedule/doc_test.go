package schedule_test

import (
	"go/doc"
	"go/parser"
	"go/token"
	"testing"
)

// TestPackageHasDocComment guards that the package carries a doc comment (AGENTS.md
// §0: simple, auditable, self-describing packages). It parses doc.go and asserts a
// non-empty package-level doc string mentioning the on/off window evaluator.
func TestPackageHasDocComment(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse dir: %v", err)
	}
	pkg, ok := pkgs["schedule"]
	if !ok {
		t.Fatalf("package schedule not found in current dir; got %v", keys(pkgs))
	}
	d := doc.New(pkg, "github.com/Principe1218/phantomTraffic/internal/schedule", 0)
	if d.Doc == "" {
		t.Fatal("package schedule has no doc comment; add one in doc.go")
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
