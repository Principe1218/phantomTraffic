package secret_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/secret"
)

// countingSource is a fake CredentialSource that records how many times Resolve
// is called, so a test can assert resolution is lazy (zero calls until asked).
type countingSource struct {
	calls   int
	secrets map[string]string // id -> plaintext
}

func (c *countingSource) Resolve(ctx context.Context, ref secret.CredentialRef) (*secret.Secret, error) {
	c.calls++
	pt, ok := c.secrets[ref.ID()]
	if !ok {
		return nil, fmt.Errorf("no secret for ref %s", ref)
	}
	return secret.New([]byte(pt)), nil
}

func TestCredentialSource_ResolvesLazily(t *testing.T) {
	src := &countingSource{secrets: map[string]string{"deploy-key": "PRIVATE-KEY-BYTES"}}
	ref, err := secret.NewCredentialRef(secret.RefKindSSHKey, "deploy-key")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Holding the ref + source (engine/behavior wiring) must NOT resolve anything.
	var _ secret.CredentialSource = src
	if src.calls != 0 {
		t.Fatalf("Resolve called %d times before any handler asked; want 0 (lazy)", src.calls)
	}

	// Only when a handler explicitly resolves does the lookup happen.
	sec, err := src.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if src.calls != 1 {
		t.Fatalf("Resolve calls = %d, want 1", src.calls)
	}

	// The resolved Secret exposes the real bytes to the handler...
	if got := string(sec.Expose()); got != "PRIVATE-KEY-BYTES" {
		t.Fatalf("resolved Expose() = %q", got)
	}
	// ...but still redacts in every rendered form.
	if rendered := fmt.Sprintf("%v %s %+v", sec, sec, sec); strings.Contains(rendered, "PRIVATE-KEY-BYTES") {
		t.Fatalf("resolved Secret leaked under fmt: %q", rendered)
	}
}

func TestCredentialSource_ResolveError(t *testing.T) {
	src := &countingSource{secrets: map[string]string{}}
	ref, err := secret.NewCredentialRef(secret.RefKindToken, "missing")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	sec, err := src.Resolve(context.Background(), ref)
	if err == nil {
		t.Fatalf("Resolve of missing secret returned %v, want error", sec)
	}
	if sec != nil {
		t.Fatalf("on error, Secret must be nil, got %v", sec)
	}
	// The error must mention the ref label but never any secret bytes (there are none here).
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error %q should reference the ref id", err.Error())
	}
	_ = errors.Unwrap(err)
}
