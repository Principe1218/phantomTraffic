package protocols

import (
	"fmt"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// RoutingError is returned by As[T] (design §2) when an Action's concrete
// type does not match the requested handler type. It maps to
// pterr.ClassPermanent: a mis-routed action is never retried. The message
// is born redacted — it carries only the action kind and protocol, never
// params, bodies, or credentials (AGENTS.md §3.1, §5.5).
type RoutingError struct {
	Got   ActionKind // the kind that was presented
	Proto ProtocolID // the protocol that was asked to handle it ("" if unknown)
	Msg   string     // optional, non-revealing context
}

func (e *RoutingError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("protocols: routing error: action %q (proto %q): %s", e.Got, e.Proto, e.Msg)
	}
	return fmt.Sprintf("protocols: routing error: action %q (proto %q)", e.Got, e.Proto)
}

// Class buckets a routing failure as permanent (do not retry).
func (e *RoutingError) Class() pterr.Class { return pterr.ClassPermanent }

// ValidationError is returned by Action.Validate (design §2) when params
// fail the entry-point allowlist BEFORE any I/O (AGENTS.md §5.2). It maps
// to pterr.ClassConfig: a bad action fails fast at validate, never starts.
// Field/Msg are non-revealing (field name + reason; never the bad value
// if that value could be a secret).
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("protocols: validation error: field %q: %s", e.Field, e.Msg)
}

// Class buckets a validation failure as config (fail fast at validate).
func (e *ValidationError) Class() pterr.Class { return pterr.ClassConfig }
