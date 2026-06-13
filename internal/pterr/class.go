package pterr

// Class is the coarse, non-revealing error bucket the engine reacts to.
// It is safe to log and to surface to the UI. The integer values are stable
// (see TestClassValuesAreStable) because Result.ErrClass uses them as a label.
type Class uint8

const (
	// ClassTransient is a temporary failure (5xx, SERVFAIL, reset, timeout):
	// bounded retry with backoff; on exhaustion a recorded failure.
	ClassTransient Class = iota
	// ClassPermanent is a non-retryable failure (4xx, NXDOMAIN, auth rejected,
	// cert/host-key invalid, off-allowlist navigation refused): record and,
	// after N consecutive, open that target's circuit.
	ClassPermanent
	// ClassConfig is a configuration/validation failure (bad YAML, unknown
	// protocol, missing known_hosts entry, rejected ApplyPatch structural
	// change): fail fast at validate/init; never start the run.
	ClassConfig
	// ClassSafety is a safety-control failure (cap exceeded, tripwire latched,
	// panic-storm, per-session backstop exceeded): latch the run into safe-stop,
	// highest precedence, no auto-reset.
	ClassSafety
	// ClassUnknown is an unclassified error escaping a handler: logged loudly and
	// treated as transient-with-reduced-budget so a sloppy handler degrades
	// gracefully instead of hot-looping.
	ClassUnknown
)

// String returns a stable, low-cardinality, non-revealing label for the class.
// Any out-of-range value maps to "unknown" so a corrupt value can never widen
// the logged surface.
func (c Class) String() string {
	switch c {
	case ClassTransient:
		return "transient"
	case ClassPermanent:
		return "permanent"
	case ClassConfig:
		return "config"
	case ClassSafety:
		return "safety"
	default:
		return "unknown"
	}
}
