package pterr

import (
	"errors"
	"fmt"
	"log/slog"
)

// Error is PhantomTraffic's structured error envelope (design §6). The exported
// fields are the redacted, safe-to-surface face of an error; the unexported
// cause is the server-side-only original.
//
//   - Class      coarse, non-revealing bucket the engine branches on.
//   - Code       stable, low-cardinality code (e.g. "http.5xx", "dns.servfail").
//   - Op         the logical operation that failed (e.g. "ssh.connect").
//   - Msg        BORN REDACTED: safe for logs and the UI; must contain no
//     secrets, headers, bodies, or stack text (AGENTS.md §3.1, §5.5).
//   - cause      the wrapped original error. SERVER-SIDE ONLY: reachable via
//     Unwrap / errors.Is / errors.As, but NEVER rendered by Error()
//     or LogValue(), so its (possibly sensitive) text cannot reach a
//     log line or the UI.
//   - retryable  whether the engine should retry; defaults from Class.
type Error struct {
	Class     Class
	Code      string
	Op        string
	Msg       string
	cause     error
	retryable bool
}

// New constructs a redacted *Error with no wrapped cause. retryable defaults
// from the class (ClassTransient and ClassUnknown retry; the rest do not).
func New(class Class, code, op, msg string) *Error {
	return &Error{
		Class:     class,
		Code:      code,
		Op:        op,
		Msg:       msg,
		retryable: defaultRetryable(class),
	}
}

// Wrap constructs a redacted *Error that wraps cause server-side. The cause is
// reachable only via Unwrap/errors.Is/errors.As — never via Error()/LogValue().
func Wrap(class Class, code, op, msg string, cause error) *Error {
	e := New(class, code, op, msg)
	e.cause = cause
	return e
}

// defaultRetryable maps a class to its default retry disposition. ClassUnknown
// is treated as transient-with-reduced-budget (design §6), so it retries.
func defaultRetryable(class Class) bool {
	switch class {
	case ClassTransient, ClassUnknown:
		return true
	default:
		return false
	}
}

// Error renders ONLY the redacted face: Class, Code, Op, and Msg. The wrapped
// cause is deliberately NOT included, so the cause's (possibly sensitive) text
// cannot leak into a log line, a wrapped %w/%v render, or the UI.
func (e *Error) Error() string {
	return fmt.Sprintf("%s [%s] %s: %s", e.Class, e.Code, e.Op, e.Msg)
}

// Unwrap exposes the wrapped cause to errors.Is/errors.As for server-side
// inspection. This is the single seam by which the cause is reachable; it is
// never formatted into a string.
func (e *Error) Unwrap() error { return e.cause }

// Retryable reports the engine's retry disposition for this error.
func (e *Error) Retryable() bool { return e.retryable }

// LogValue implements slog.LogValuer so structured logging emits ONLY the
// redacted fields. The cause is structurally excluded here, so a credential or
// any sensitive cause text can never be formatted into a log record via this
// error (AGENTS.md §3.1, §5.5).
func (e *Error) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("class", e.Class.String()),
		slog.String("code", e.Code),
		slog.String("op", e.Op),
		slog.String("msg", e.Msg),
		slog.Bool("retryable", e.retryable),
	)
}

// Classify returns the Class of the first *Error in err's tree, or ClassUnknown
// if err is nil or no *Error is present. This is how the engine collapses a
// handler's long failure tail to a single class to branch on (design §6).
func Classify(err error) Class {
	if err == nil {
		return ClassUnknown
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Class
	}
	return ClassUnknown
}

// IsClass reports whether err's tree contains an *Error of class c.
func IsClass(err error, c Class) bool {
	return Classify(err) == c
}

// compile-time guarantees that *Error satisfies the contract interfaces.
var (
	_ error          = (*Error)(nil)
	_ slog.LogValuer = (*Error)(nil)
)
