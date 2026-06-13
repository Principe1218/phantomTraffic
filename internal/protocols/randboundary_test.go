package protocols

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoMathRandImports enforces the design §7 crypto/rand vs math/rand
// separation: internal/protocols is a non-security contract layer but is NOT
// one of the two packages (internal/rng, internal/behavior) permitted to import
// math/rand. Any draw of randomness here would be a determinism/security smell.
// This guard parses every .go file in the package and fails on a math/rand or
// math/rand/v2 import — a local, fast backstop to the repo-wide forbidigo lint.
func TestNoMathRandImports(t *testing.T) {
	banned := map[string]bool{
		"math/rand":    true,
		"math/rand/v2": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		path := filepath.Join(".", name)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", path, err)
			}
			if banned[p] {
				t.Errorf("%s imports %q — forbidden in internal/protocols (math/rand allowed only in internal/rng and internal/behavior; design §7, AGENTS.md §2.2)", path, p)
			}
		}
	}
	_ = ast.NewIdent // keep go/ast referenced for import stability
}
