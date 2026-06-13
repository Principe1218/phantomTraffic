package secret

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// maxRefIDLen bounds the credential ID label (AGENTS.md §5.2 — bounded input).
const maxRefIDLen = 256

// RefKind enumerates the allowed categories of credential a CredentialRef may
// point at. It is an allowlist: NewCredentialRef rejects any other value.
type RefKind uint8

const (
	RefKindSSHKey   RefKind = iota // an SSH private key (resolved to key bytes)
	RefKindPassword                // a password
	RefKindToken                   // a bearer/API token
	RefKindEnvVar                  // a named environment variable holding the secret
)

// valid reports whether k is one of the allowlisted RefKind values.
func (k RefKind) valid() bool {
	switch k {
	case RefKindSSHKey, RefKindPassword, RefKindToken, RefKindEnvVar:
		return true
	default:
		return false
	}
}

// String renders the kind as a stable lowercase label for logs/audit.
func (k RefKind) String() string {
	switch k {
	case RefKindSSHKey:
		return "ssh_key"
	case RefKindPassword:
		return "password"
	case RefKindToken:
		return "token"
	case RefKindEnvVar:
		return "env_var"
	default:
		return "unknown"
	}
}

// CredentialRef is an OPAQUE handle naming a credential. It carries NO secret
// bytes — only a non-secret Kind + ID label — so it is safe to store on a
// Target and stream to the UI. (There is no MarshalYAML method yet, so YAML
// serialization is not specially guaranteed by this type.) Secret material is
// resolved lazily, inside a protocol handler, via CredentialSource.Resolve.
type CredentialRef struct {
	kind RefKind
	id   string
}

// NewCredentialRef builds a validated CredentialRef. The id is a non-secret
// label (e.g. "deploy-key", or an env-var name). It must be non-empty after
// trimming, within maxRefIDLen, contain no control characters, and the kind
// must be allowlisted. On any violation it returns the zero CredentialRef and
// a non-nil error.
func NewCredentialRef(kind RefKind, id string) (CredentialRef, error) {
	if !kind.valid() {
		return CredentialRef{}, fmt.Errorf("secret: invalid credential kind %d", uint8(kind))
	}
	if strings.TrimSpace(id) == "" {
		return CredentialRef{}, fmt.Errorf("secret: credential id must be non-empty")
	}
	if len(id) > maxRefIDLen {
		return CredentialRef{}, fmt.Errorf("secret: credential id exceeds %d bytes", maxRefIDLen)
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return CredentialRef{}, fmt.Errorf("secret: credential id contains control character")
		}
	}
	return CredentialRef{kind: kind, id: id}, nil
}

// ID returns the non-secret credential label.
func (r CredentialRef) ID() string { return r.id }

// Kind returns the credential category.
func (r CredentialRef) Kind() RefKind { return r.kind }

// IsZero reports whether r is the zero value (no credential).
func (r CredentialRef) IsZero() bool { return r.id == "" }

// String renders a stable, non-secret representation: credref(<kind>:<id>).
func (r CredentialRef) String() string {
	if r.IsZero() {
		return "credref(none)"
	}
	return fmt.Sprintf("credref(%s:%s)", r.kind.String(), r.id)
}

// LogValue implements slog.LogValuer so a CredentialRef logs as structured,
// non-secret fields (kind + id label), never anything resolvable to bytes.
func (r CredentialRef) LogValue() slog.Value {
	if r.IsZero() {
		return slog.StringValue("credref(none)")
	}
	return slog.GroupValue(
		slog.String("kind", r.kind.String()),
		slog.String("id", r.id),
	)
}

// CredentialSource resolves a CredentialRef to its Secret material LAZILY.
// Implementations (env var, secrets manager, prompt) perform the lookup only
// when Resolve is called — never at wiring time. Resolve is invoked ONLY inside
// a protocol handler; the engine and behavior layers hold the interface and a
// CredentialRef, never the resolved Secret (design §2 SessionDeps.CredSrc).
type CredentialSource interface {
	Resolve(ctx context.Context, ref CredentialRef) (*Secret, error)
}
