package pterr

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

const sensitiveCause = "password=hunter2 token=AKIAEXAMPLESECRET"

func TestNewBuildsRedactedEnvelope(t *testing.T) {
	e := New(ClassConfig, "scenario.bad_yaml", "scenario.Validate", "scenario failed validation")
	if e.Class != ClassConfig {
		t.Fatalf("Class = %v, want ClassConfig", e.Class)
	}
	if e.Code != "scenario.bad_yaml" {
		t.Fatalf("Code = %q, want scenario.bad_yaml", e.Code)
	}
	if e.Op != "scenario.Validate" {
		t.Fatalf("Op = %q, want scenario.Validate", e.Op)
	}
	if e.Msg != "scenario failed validation" {
		t.Fatalf("Msg = %q, want 'scenario failed validation'", e.Msg)
	}
	if e.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil for New", e.Unwrap())
	}
}

func TestErrorStringNeverLeaksCause(t *testing.T) {
	cause := errors.New(sensitiveCause)
	e := Wrap(ClassTransient, "ssh.dial_timeout", "ssh.connect", "connection failed", cause)

	got := e.Error()
	// The public string MUST carry the redacted Msg + Op + Code...
	if !strings.Contains(got, "connection failed") {
		t.Fatalf("Error() = %q, want it to contain the redacted Msg", got)
	}
	if !strings.Contains(got, "ssh.connect") {
		t.Fatalf("Error() = %q, want it to contain the Op", got)
	}
	if !strings.Contains(got, "ssh.dial_timeout") {
		t.Fatalf("Error() = %q, want it to contain the Code", got)
	}
	// ...but NEVER the wrapped cause's sensitive text.
	if strings.Contains(got, "hunter2") || strings.Contains(got, "AKIAEXAMPLESECRET") || strings.Contains(got, sensitiveCause) {
		t.Fatalf("Error() leaked the wrapped cause: %q", got)
	}
}

func TestUnwrapExposesCauseServerSideOnly(t *testing.T) {
	cause := errors.New(sensitiveCause)
	e := Wrap(ClassPermanent, "ssh.auth_rejected", "ssh.connect", "authentication failed", cause)

	// Unwrap is the server-side-only seam: errors.Is/As can still reach the cause.
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is(e, cause) = false, want true (Unwrap must expose cause to errors.Is)")
	}
	if e.Unwrap() != cause {
		t.Fatalf("Unwrap() = %v, want the wrapped cause", e.Unwrap())
	}
}

func TestErrorsAsRecoversConcreteType(t *testing.T) {
	cause := errors.New("io: read on closed pipe")
	wrapped := Wrap(ClassTransient, "http.5xx", "http.request", "upstream error", cause)

	var pe *Error
	if !errors.As(wrapped, &pe) {
		t.Fatal("errors.As(wrapped, &*Error) = false, want true")
	}
	if pe.Code != "http.5xx" {
		t.Fatalf("recovered Code = %q, want http.5xx", pe.Code)
	}
}

func TestRetryableTracksClassByDefault(t *testing.T) {
	if !New(ClassTransient, "x", "op", "m").Retryable() {
		t.Fatal("ClassTransient default Retryable() = false, want true")
	}
	if New(ClassPermanent, "x", "op", "m").Retryable() {
		t.Fatal("ClassPermanent default Retryable() = true, want false")
	}
	if New(ClassConfig, "x", "op", "m").Retryable() {
		t.Fatal("ClassConfig default Retryable() = true, want false")
	}
	if New(ClassSafety, "x", "op", "m").Retryable() {
		t.Fatal("ClassSafety default Retryable() = true, want false")
	}
	// ClassUnknown is treated as transient-with-reduced-budget (design §6) ⇒ retryable.
	if !New(ClassUnknown, "x", "op", "m").Retryable() {
		t.Fatal("ClassUnknown default Retryable() = false, want true")
	}
}

func TestClassifyAndIsClass(t *testing.T) {
	plain := errors.New("not a pterr error")
	if got := Classify(plain); got != ClassUnknown {
		t.Fatalf("Classify(plain) = %v, want ClassUnknown", got)
	}
	if got := Classify(nil); got != ClassUnknown {
		t.Fatalf("Classify(nil) = %v, want ClassUnknown", got)
	}

	safety := New(ClassSafety, "safety.cap_exceeded", "limiter.Acquire", "rate cap exceeded")
	if got := Classify(safety); got != ClassSafety {
		t.Fatalf("Classify(safety) = %v, want ClassSafety", got)
	}
	if !IsClass(safety, ClassSafety) {
		t.Fatal("IsClass(safety, ClassSafety) = false, want true")
	}
	if IsClass(safety, ClassTransient) {
		t.Fatal("IsClass(safety, ClassTransient) = true, want false")
	}

	// Classify reaches a *Error wrapped inside a stdlib error chain.
	cause := New(ClassPermanent, "dns.nxdomain", "dns.query", "no such domain")
	outer := errors.New("wrapper")
	chained := errorsJoin(outer, cause) // helper below: cause is reachable via Unwrap tree
	if got := Classify(chained); got != ClassPermanent {
		t.Fatalf("Classify(chained) = %v, want ClassPermanent", got)
	}
	if !IsClass(chained, ClassPermanent) {
		t.Fatal("IsClass(chained, ClassPermanent) = false, want true")
	}
}

// errorsJoin builds a genuine 2-element joined error so errors.As/Is must traverse
// a real multi-error Unwrap tree (errors.Join's Unwrap() []error form) to reach inner.
func errorsJoin(outer error, inner error) error {
	return errors.Join(outer, inner)
}

func TestLogValueNeverLeaksCauseThroughSlog(t *testing.T) {
	cause := errors.New(sensitiveCause)
	e := Wrap(ClassPermanent, "ssh.auth_rejected", "ssh.connect", "authentication failed", cause)

	var buf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Error("ssh connect failed", slog.Any("err", e))

	out := buf.String()

	// The redacted face must be present...
	if !strings.Contains(out, "authentication failed") {
		t.Fatalf("structured log missing redacted Msg; got: %s", out)
	}
	if !strings.Contains(out, "ssh.auth_rejected") {
		t.Fatalf("structured log missing Code; got: %s", out)
	}
	// ...and the sensitive cause text must NEVER appear in the rendered record.
	if strings.Contains(out, "hunter2") || strings.Contains(out, "AKIAEXAMPLESECRET") || strings.Contains(out, sensitiveCause) {
		t.Fatalf("structured log LEAKED the wrapped cause: %s", out)
	}

	// Even so, the cause stays reachable server-side via errors.Is.
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is(e, cause) = false; cause must remain reachable server-side")
	}
}
