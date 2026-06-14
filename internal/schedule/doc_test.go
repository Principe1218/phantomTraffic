package schedule_test

import (
	"go/ast"
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
	f, err := parser.ParseFile(fset, "doc.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse doc.go: %v", err)
	}
	d, err := doc.NewFromFiles(fset, []*ast.File{f}, "github.com/Principe1218/phantomTraffic/internal/schedule")
	if err != nil {
		t.Fatalf("doc.NewFromFiles: %v", err)
	}
	if d.Doc == "" {
		t.Fatal("package schedule has no doc comment; add one in doc.go")
	}
}
