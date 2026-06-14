package engine

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestRunGuardedNoPanic verifies a clean function reports panicked=false and the
// function actually ran.
func TestRunGuardedNoPanic(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	ran := false
	panicked := runGuarded(log, func() { ran = true })
	if panicked {
		t.Fatal("runGuarded reported panicked=true for a clean function")
	}
	if !ran {
		t.Fatal("runGuarded did not invoke the function")
	}
}

// TestRunGuardedPanic verifies a panicking function is recovered, reported as
// panicked=true, and logged at Error WITHOUT leaking the raw stack into the
// message text (redaction: design §7.2 / AGENTS.md §5.5).
func TestRunGuardedPanic(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	panicked := runGuarded(log, func() { panic("boom: secret=hunter2") })
	if !panicked {
		t.Fatal("runGuarded reported panicked=false for a panicking function")
	}

	// Exactly one Error record must have been emitted.
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("runGuarded did not log the recovered panic")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("log record is not valid JSON: %v\n%s", err, out)
	}
	if lvl, _ := rec["level"].(string); lvl != "ERROR" {
		t.Fatalf("recovered panic logged at level %q, want ERROR", lvl)
	}
	// The log line must NOT contain a raw goroutine stack dump. A real stack from
	// runtime/debug.Stack() contains the literal "goroutine " marker; assert it is
	// absent so a stack (which can carry sensitive frame data) never reaches a log.
	if strings.Contains(out, "goroutine ") {
		t.Fatalf("recovered-panic log leaked a raw goroutine stack:\n%s", out)
	}
}
