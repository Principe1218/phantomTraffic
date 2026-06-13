package protocols

import "fmt"

// Action is the open marker interface every protocol action implements.
// Engine and behavior stay MONOMORPHIC — they program against Action and
// never see concrete fields (so credentials/bodies cannot leak upward,
// AGENTS.md §5.2). Handlers regain full static typing via As[T]. There is
// NO unexported marker method: Action is intentionally open so out-of-tree
// protocol packages can implement it and register a handler without
// modifying core (design §2 extensibility decision). Exhaustiveness comes
// from (1) the Registry enumerating handlers at wiring time and (2) every
// action flowing through exactly ONE audited As[T] cast site per kind.
type Action interface {
	Kind() ActionKind
	Protocol() ProtocolID
	Cause() Cause       // drives think-time applicability
	Pacing() PacingMode // drives who owns inter-action timing
	Validate() error    // allowlist params BEFORE any I/O; returns *ValidationError
}

// Cause classifies WHY an action happens, which the behavior chainShaper
// reads to decide whether a human pause applies (design §2, §4).
type Cause uint8

const (
	CauseNavigation  Cause = iota // human-initiated; full think-time applies
	CauseSubResource              // automatic (CSS/JS/img, A+AAAA fan-out); near-zero gap
	CauseBackground               // recurring beacon/poll; own cadence
	CauseControl                  // open/close/handshake; no human gap
)

func (c Cause) String() string {
	switch c {
	case CauseNavigation:
		return "navigation"
	case CauseSubResource:
		return "sub-resource"
	case CauseBackground:
		return "background"
	case CauseControl:
		return "control"
	default:
		return fmt.Sprintf("cause(%d)", uint8(c))
	}
}

// PacingMode decides who owns inter-action timing (design §2, §4).
type PacingMode uint8

const (
	PacingShaperManaged PacingMode = iota // behavior chainShaper computes Wait (HTTP/SSH/DNS default)
	PacingSelfPaced                       // the action/session computes Wait internally (streaming buffer clock)
)

func (p PacingMode) String() string {
	switch p {
	case PacingShaperManaged:
		return "shaper-managed"
	case PacingSelfPaced:
		return "self-paced"
	default:
		return fmt.Sprintf("pacing(%d)", uint8(p))
	}
}

// BaseAction is embedded by every concrete action struct to supply the
// Protocol/Cause/Pacing accessors. Concrete structs add Kind() and
// Validate(). (Kind is per-struct because it is the verb identity.)
type BaseAction struct {
	Proto ProtocolID
	C     Cause
	P     PacingMode
}

func (b BaseAction) Protocol() ProtocolID { return b.Proto }
func (b BaseAction) Cause() Cause         { return b.C }
func (b BaseAction) Pacing() PacingMode   { return b.P }

// As is the single, audited downcast site per action type (design §2). No
// inline type assertions are permitted anywhere else: a handler calls
// As[ConcreteAction] exactly once at the top of Do. On a type mismatch it
// returns a *RoutingError (pterr.ClassPermanent) instead of panicking —
// an unrecognized Action is a routing failure, not a compile break, which
// keeps "extensible = add a package, register a handler" true.
func As[T Action](a Action) (T, error) {
	var zero T
	// Guard the nil interface: a nil Action reaching a cast site is an engine bug,
	// but As is the single audited downcast and its "never panics" contract is
	// absolute — without this, the mismatch branch's a.Kind()/a.Protocol() calls
	// would nil-dereference and surface as an OutcomePanicked deep in a handler.
	if a == nil {
		return zero, &RoutingError{Msg: "nil action presented to handler"}
	}
	t, ok := a.(T)
	if !ok {
		return zero, &RoutingError{Got: a.Kind(), Proto: a.Protocol(), Msg: "action concrete type does not match handler"}
	}
	return t, nil
}
