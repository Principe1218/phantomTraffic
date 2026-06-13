// Package secret holds in-memory secret material behind a wrapper that can never
// be formatted into a log line, a fmt verb, or JSON. See AGENTS.md §3.1.
package secret

import (
	"encoding/json"
	"log/slog"
)

// Redacted is the single rendered placeholder for any secret value. Every
// formatting and serialization path on Secret returns exactly this string, so a
// secret can never be printed, logged (slog), or JSON-encoded in the clear.
const Redacted = "[REDACTED]"

// Secret wraps raw secret bytes (an SSH key, password, or token) in memory.
// The zero value is a valid empty secret. A Secret is reference-like: it holds
// a single backing slice and is intended to be passed by pointer so Zero() can
// wipe the one copy. It is NOT safe for concurrent mutation (Zero vs Expose).
type Secret struct {
	b []byte
}

// New copies b into a new Secret. The caller may safely zero or reuse b after.
func New(b []byte) *Secret {
	cp := make([]byte, len(b))
	copy(cp, b)
	return &Secret{b: cp}
}

// Expose returns the backing secret bytes for legitimate handler use ONLY
// (e.g. parsing an SSH private key inside a protocol handler). Callers MUST NOT
// log, copy into long-lived structures, or format the result. The slice aliases
// the Secret's storage; do not retain it past the handler call.
func (s *Secret) Expose() []byte {
	if s == nil {
		return nil
	}
	return s.b
}

// Len reports the secret length in bytes without exposing its contents.
func (s *Secret) Len() int {
	if s == nil {
		return 0
	}
	return len(s.b)
}

// Zero is a best-effort wipe of the backing bytes. It is documented as
// best-effort: the Go runtime may have copied the bytes elsewhere (GC moves,
// io buffers), so this is defense-in-depth, not a guarantee (AGENTS.md §3.1).
func (s *Secret) Zero() {
	if s == nil {
		return
	}
	for i := range s.b {
		s.b[i] = 0
	}
	s.b = nil
}

// String implements fmt.Stringer so %v and %s render the placeholder.
func (s *Secret) String() string { return Redacted }

// GoString implements fmt.GoStringer so %#v renders the placeholder.
func (s *Secret) GoString() string { return Redacted }

// LogValue implements slog.LogValuer so slog ALWAYS records the placeholder,
// even when a Secret is embedded in a logged struct (slog.Any walks LogValuer).
func (s *Secret) LogValue() slog.Value { return slog.StringValue(Redacted) }

// MarshalJSON implements json.Marshaler so a Secret serialized to JSON (e.g. a
// config dump streamed to the UI) emits the placeholder string, never bytes.
func (s *Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(Redacted)
}
