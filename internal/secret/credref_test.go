package secret_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/secret"
)

func TestNewCredentialRef_Valid(t *testing.T) {
	cases := []struct {
		name string
		kind secret.RefKind
		id   string
	}{
		{"ssh_key", secret.RefKindSSHKey, "deploy-key"},
		{"password", secret.RefKindPassword, "svc-account"},
		{"token", secret.RefKindToken, "api-bearer"},
		{"env_var", secret.RefKindEnvVar, "PT_DB_PASSWORD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := secret.NewCredentialRef(tc.kind, tc.id)
			if err != nil {
				t.Fatalf("NewCredentialRef(%v,%q) error: %v", tc.kind, tc.id, err)
			}
			if ref.ID() != tc.id {
				t.Fatalf("ID() = %q, want %q", ref.ID(), tc.id)
			}
			if ref.Kind() != tc.kind {
				t.Fatalf("Kind() = %v, want %v", ref.Kind(), tc.kind)
			}
			if ref.IsZero() {
				t.Fatalf("valid ref reported IsZero()")
			}
		})
	}
}

func TestNewCredentialRef_Invalid(t *testing.T) {
	longID := strings.Repeat("a", 257)
	cases := []struct {
		name string
		kind secret.RefKind
		id   string
	}{
		{"empty_id", secret.RefKindSSHKey, ""},
		{"whitespace_id", secret.RefKindToken, "   "},
		{"too_long_id", secret.RefKindPassword, longID},
		{"unknown_kind", secret.RefKind(200), "x"},
		{"control_char_id", secret.RefKindToken, "bad\nid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := secret.NewCredentialRef(tc.kind, tc.id)
			if err == nil {
				t.Fatalf("NewCredentialRef(%v,%q) = %v, want error", tc.kind, tc.id, ref)
			}
			if !ref.IsZero() {
				t.Fatalf("on error, returned ref must be zero, got %v", ref)
			}
		})
	}
}

func TestCredentialRef_ZeroValue(t *testing.T) {
	var ref secret.CredentialRef
	if !ref.IsZero() {
		t.Fatalf("zero-value CredentialRef should report IsZero()")
	}
}

func TestCredentialRef_RendersSafely(t *testing.T) {
	ref, err := secret.NewCredentialRef(secret.RefKindSSHKey, "deploy-key")
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}
	// String/log render the non-secret label, in a stable, recognizable shape.
	got := ref.String()
	if !strings.Contains(got, "deploy-key") {
		t.Fatalf("String() = %q, want it to contain the id label", got)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("resolving", slog.Any("cred", ref))
	out := buf.String()
	if !strings.Contains(out, "deploy-key") {
		t.Fatalf("slog of CredentialRef dropped the id label: %s", out)
	}
}
