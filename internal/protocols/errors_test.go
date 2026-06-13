package protocols

import (
	"errors"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

func TestRoutingError_ClassAndMessage(t *testing.T) {
	err := &RoutingError{Got: "fetch-page", Proto: "http", Msg: "wrong concrete type"}
	if got := err.Class(); got != pterr.ClassPermanent {
		t.Fatalf("RoutingError.Class() = %v, want %v", got, pterr.ClassPermanent)
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("RoutingError.Error() must be non-empty")
	}
	// Must not be confused with a generic error: still satisfies error.
	var e error = err
	if !errors.As(e, new(*RoutingError)) {
		t.Fatal("errors.As must recover *RoutingError")
	}
}

func TestValidationError_ClassAndMessage(t *testing.T) {
	err := &ValidationError{Field: "Addr", Msg: "port out of range"}
	if got := err.Class(); got != pterr.ClassConfig {
		t.Fatalf("ValidationError.Class() = %v, want %v", got, pterr.ClassConfig)
	}
	if err.Error() == "" {
		t.Fatal("ValidationError.Error() must be non-empty")
	}
}
