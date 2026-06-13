package secret_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/secret"
)

// plaintext is the sensitive value that must never appear in any rendered form.
const plaintext = "hunter2-super-secret-token"

func TestSecret_FmtNeverLeaks(t *testing.T) {
	s := secret.New([]byte(plaintext))

	cases := []struct {
		name   string
		format string
	}{
		{"verb_v", "%v"},
		{"verb_s", "%s"},
		{"verb_plus_v", "%+v"},
		{"verb_hash_v", "%#v"},
		{"verb_q", "%q"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sprintf(tc.format, s)
			if strings.Contains(got, plaintext) {
				t.Fatalf("format %q leaked the secret: %q", tc.format, got)
			}
			if !strings.Contains(got, secret.Redacted) {
				t.Fatalf("format %q did not render %q; got %q", tc.format, secret.Redacted, got)
			}
		})
	}
}

func TestSecret_SlogNeverLeaks(t *testing.T) {
	s := secret.New([]byte(plaintext))

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Embed the Secret in a struct field, the realistic leak path.
	type creds struct {
		User string
		Pass *secret.Secret
	}
	logger.Info("auth attempt",
		slog.String("user", "alice"),
		slog.Any("password", s),
		slog.Any("creds", creds{User: "alice", Pass: s}),
	)

	out := buf.String()
	if strings.Contains(out, plaintext) {
		t.Fatalf("slog leaked the secret: %s", out)
	}
	if !strings.Contains(out, secret.Redacted) {
		t.Fatalf("slog did not emit %q; got %s", secret.Redacted, out)
	}
}

func TestSecret_ExposeReturnsRealBytes(t *testing.T) {
	s := secret.New([]byte(plaintext))
	if got := string(s.Expose()); got != plaintext {
		t.Fatalf("Expose() = %q, want %q", got, plaintext)
	}
	if got := s.Len(); got != len(plaintext) {
		t.Fatalf("Len() = %d, want %d", got, len(plaintext))
	}
}

func TestSecret_JSONNeverLeaks(t *testing.T) {
	type creds struct {
		User string         `json:"user"`
		Pass *secret.Secret `json:"pass"`
	}
	s := secret.New([]byte(plaintext))

	out, err := json.Marshal(creds{User: "alice", Pass: s})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	got := string(out)
	if strings.Contains(got, plaintext) {
		t.Fatalf("json.Marshal leaked the secret: %s", got)
	}
	if !strings.Contains(got, secret.Redacted) {
		t.Fatalf("json.Marshal did not emit %q; got %s", secret.Redacted, got)
	}
	// The marshaled value must be a JSON string literal, not raw bytes.
	if !strings.Contains(got, `"pass":"`+secret.Redacted+`"`) {
		t.Fatalf("json field shape unexpected: %s", got)
	}
}

func TestSecret_ZeroWipesBytes(t *testing.T) {
	s := secret.New([]byte(plaintext))
	b := s.Expose() // aliases backing storage
	if string(b) != plaintext {
		t.Fatalf("precondition: Expose() = %q", string(b))
	}
	s.Zero()
	for i, c := range b {
		if c != 0 {
			t.Fatalf("Zero() left byte %d = %d (not wiped)", i, c)
		}
	}
	if s.Len() != 0 {
		t.Fatalf("after Zero(), Len() = %d, want 0", s.Len())
	}
	if s.Expose() != nil {
		t.Fatalf("after Zero(), Expose() should be nil")
	}
}

func TestSecret_NilSafe(t *testing.T) {
	var s *secret.Secret
	if s.Len() != 0 {
		t.Fatalf("nil Len() = %d, want 0", s.Len())
	}
	if s.Expose() != nil {
		t.Fatalf("nil Expose() should be nil")
	}
	s.Zero() // must not panic
}

// canary is a distinct sensitive value used by the by-value-copy regression test.
const canary = "CANARY"

// TestSecret_ByValueCopyNeverLeaks guards the value-receiver fix: a Secret copied
// BY VALUE (sv := *New(...)) and a Secret stored as a by-VALUE struct field must
// still redact under every fmt verb, slog, and json.Marshal. With pointer-receiver
// redaction methods, a value copy would not satisfy Stringer/LogValuer/Marshaler
// and would leak its bytes (AGENTS.md §3.1).
func TestSecret_ByValueCopyNeverLeaks(t *testing.T) {
	sv := *secret.New([]byte(canary)) // by-value copy of a *Secret

	// By-value struct field (not a pointer) — the silent value-JSON / value-fmt path.
	type wrapper struct {
		Name string        `json:"name"`
		Sec  secret.Secret `json:"sec"`
	}
	w := wrapper{Name: "alice", Sec: *secret.New([]byte(canary))}

	// fmt verbs over both the bare value copy and the by-value struct field.
	formats := []string{"%s", "%q", "%v", "%+v", "%#v"}
	for _, f := range formats {
		for _, arg := range []any{sv, w} {
			got := fmt.Sprintf(f, arg)
			if strings.Contains(got, canary) {
				t.Fatalf("format %q over %T leaked the canary: %q", f, arg, got)
			}
		}
		// The bare value copy must specifically render the placeholder.
		if got := fmt.Sprintf(f, sv); !strings.Contains(got, secret.Redacted) {
			t.Fatalf("format %q over value copy did not render %q; got %q", f, secret.Redacted, got)
		}
	}

	// slog with the Secret as a by-VALUE struct field.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("value-field log", slog.Any("wrapper", w), slog.Any("copy", sv))
	if out := buf.String(); strings.Contains(out, canary) {
		t.Fatalf("slog leaked the canary via value field: %s", out)
	}

	// json.Marshal with the Secret as a by-VALUE struct field.
	out, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if got := string(out); strings.Contains(got, canary) {
		t.Fatalf("json.Marshal leaked the canary via value field: %s", got)
	} else if !strings.Contains(got, secret.Redacted) {
		t.Fatalf("json.Marshal value field did not emit %q; got %s", secret.Redacted, got)
	}
}

// sprintf is a tiny indirection so the table can hold format strings.
func sprintf(format string, args ...any) string {
	return strings.TrimSpace(fmt.Sprintf(format, args...))
}
