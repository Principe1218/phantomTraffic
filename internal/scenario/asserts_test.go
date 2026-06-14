package scenario

import (
	"strings"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// assertEqual fails the test unless got == want, labeling the mismatch so a
// failure names the exact field. It collapses the repeated
// `if got != want { t.Fatalf(...) }` shape into a single call, which is what
// keeps the table- and happy-path tests under the Cognitive Complexity limit.
func assertEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

// expectEqual is the non-fatal counterpart of assertEqual: on mismatch it calls
// t.Errorf and lets the test keep running, so one run reports every field that
// decoded wrong instead of halting at the first. Use it for independent value
// checks; use assertEqual when a later step (e.g. slice indexing) depends on the
// check having passed.
func expectEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// requireValidationErrors asserts err is a non-nil pterr.FieldErrors and returns
// it. It folds the two checks every "expect a config error" test repeats — the
// non-nil guard and the type assertion — into one helper.
func requireValidationErrors(t *testing.T, err error) pterr.FieldErrors {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a FieldErrors, got nil")
	}
	ve, ok := err.(pterr.FieldErrors)
	if !ok {
		t.Fatalf("error is %T, want pterr.FieldErrors", err)
	}
	return ve
}

// assertFieldError fails unless ve contains a FieldError whose Field equals field
// and whose Msg contains msgSub. Pass msgSub == "" to match on the field path
// alone (strings.Contains(_, "") is always true).
func assertFieldError(t *testing.T, ve pterr.FieldErrors, field, msgSub string) {
	t.Helper()
	for _, fe := range ve {
		if fe.Field == field && strings.Contains(fe.Msg, msgSub) {
			return
		}
	}
	t.Fatalf("missing FieldError at %q containing %q; got %v", field, msgSub, ve)
}
