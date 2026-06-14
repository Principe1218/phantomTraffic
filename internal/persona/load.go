package persona

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"sync"

	"go.yaml.in/yaml/v3"
)

//go:embed personas/*.yaml
var builtinFS embed.FS

// DefaultPersonaName is the persona a scenario block uses when it omits persona:.
const DefaultPersonaName = "office-worker"

// loadBuiltinsOnce parses and validates every embedded persona exactly once. The
// cached map MUST NOT be mutated by callers.
var loadBuiltinsOnce = sync.OnceValues(loadBuiltins)

func loadBuiltins() (map[string]Persona, error) {
	entries, err := builtinFS.ReadDir("personas")
	if err != nil {
		return nil, fmt.Errorf("persona: read embedded dir: %w", err)
	}
	out := make(map[string]Persona, len(entries))
	for _, e := range entries {
		data, err := builtinFS.ReadFile("personas/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("persona: read %s: %w", e.Name(), err)
		}
		var raw RawPersona
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true) // a typo in a shipped built-in must fail the build's tests
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("persona: decode %s: %w", e.Name(), err)
		}
		p, err := Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("persona: compile %s: %w", e.Name(), err)
		}
		if _, dup := out[p.Name]; dup {
			return nil, fmt.Errorf("persona: duplicate built-in name %q", p.Name)
		}
		out[p.Name] = p
	}
	return out, nil
}

// Builtins returns the embedded persona library keyed by name. The result is
// cached and MUST NOT be mutated.
func Builtins() (map[string]Persona, error) { return loadBuiltinsOnce() }

// Lookup resolves a built-in persona by name. The error is non-nil only if the
// embedded library itself fails to load (a build defect caught by TestBuiltins).
func Lookup(name string) (Persona, bool, error) {
	bs, err := loadBuiltinsOnce()
	if err != nil {
		return Persona{}, false, err
	}
	p, ok := bs[name]
	return p, ok, nil
}

// BuiltinNames returns the sorted built-in names for help and error messages.
func BuiltinNames() ([]string, error) {
	bs, err := loadBuiltinsOnce()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(bs))
	for n := range bs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}
