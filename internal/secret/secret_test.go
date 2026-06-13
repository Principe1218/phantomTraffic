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

// sprintf is a tiny indirection so the table can hold format strings.
func sprintf(format string, args ...any) string {
	return strings.TrimSpace(fmt.Sprintf(format, args...))
}
