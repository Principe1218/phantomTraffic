package scenario

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

func TestLoadHappyPath(t *testing.T) {
	raw, err := Load(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("Load(valid.yaml) returned error: %v", err)
	}

	expectEqual(t, "Name", raw.Name, "smoke")
	expectEqual(t, "Description", raw.Description, "minimal valid scenario for decode tests")

	// Length checks gate the indexing below, so they must halt the test (assert*).
	assertEqual(t, "len(AllowedDomains)", len(raw.AllowedDomains), 2)
	expectEqual(t, "AllowedDomains[0]", raw.AllowedDomains[0], "example.com")
	expectEqual(t, "AllowedDomains[1]", raw.AllowedDomains[1], "api.example.com")

	// Caps decode into the typed fields with the exact yaml tags.
	expectEqual(t, "Caps.PerTargetRPS", raw.Caps.PerTargetRPS, 5)
	expectEqual(t, "Caps.GlobalRPS", raw.Caps.GlobalRPS, 25)
	expectEqual(t, "Caps.MaxConcurrentSessions", raw.Caps.MaxConcurrentSessions, 10)
	expectEqual(t, "Caps.TotalRequestBudget", raw.Caps.TotalRequestBudget, 100000)
	expectEqual(t, "Caps.StreamingByteRateKbps", raw.Caps.StreamingByteRateKbps, 6000)
	expectEqual(t, "Caps.ConcurrentStreams", raw.Caps.ConcurrentStreams, 2)
	expectEqual(t, "Caps.PerSessionMaxDurationSeconds", raw.Caps.PerSessionMaxDurationSeconds, 600)
	expectEqual(t, "Caps.PerSessionMaxActions", raw.Caps.PerSessionMaxActions, 5000)

	// Execution.
	expectEqual(t, "Execution.Mode", raw.Execution.Mode, "sequential")
	expectEqual(t, "Execution.StopOnError", raw.Execution.StopOnError, true)

	// Scenario blocks.
	assertEqual(t, "len(Scenarios)", len(raw.Scenarios), 2)
	first := raw.Scenarios[0]
	expectEqual(t, "Scenarios[0].ID", first.ID, "web-browse")
	expectEqual(t, "Scenarios[0].Protocol", first.Protocol, "http")
	assertEqual(t, "len(Scenarios[0].Targets)", len(first.Targets), 2)
	expectEqual(t, "Scenarios[0].Targets[0]", first.Targets[0], "example.com:443")
	expectEqual(t, "Scenarios[0].TargetRotation", first.TargetRotation, "random")
	expectEqual(t, "Scenarios[0].TargetRotationIntervalSeconds", first.TargetRotationIntervalSeconds, 30)
	expectEqual(t, "Scenarios[0].AllowInsecure", first.AllowInsecure, false)

	second := raw.Scenarios[1]
	expectEqual(t, "Scenarios[1].ID", second.ID, "name-lookups")
	expectEqual(t, "Scenarios[1].Protocol", second.Protocol, "dns")
	expectEqual(t, "Scenarios[1].TargetRotationIntervalSeconds", second.TargetRotationIntervalSeconds, 0)
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "unknown_key.yaml"))
	if err == nil {
		t.Fatal("Load(unknown_key.yaml) = nil error, want a strict-decode failure")
	}

	// The error must classify as ClassConfig via the *pterr.Error envelope.
	var pe *pterr.Error
	if !errors.As(err, &pe) {
		t.Fatalf("error is not a *pterr.Error: %v", err)
	}
	if pe.Class != pterr.ClassConfig {
		t.Fatalf("error Class = %v, want ClassConfig", pe.Class)
	}

	// The decoder's underlying message should mention the offending field so an
	// operator can find the typo. yaml.v3 reports it as "field allow_insecure not found".
	cause := pe.Unwrap()
	if cause == nil {
		t.Fatal("expected the *pterr.Error to wrap the decoder's cause")
	}
	if !strings.Contains(cause.Error(), "allow_insecure") {
		t.Fatalf("wrapped cause = %q, want it to mention the unknown field allow_insecure", cause.Error())
	}
}

func TestLoadRejectsOversizeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.yaml")

	// Build a file just over the 1 MiB cap. The size check must fire BEFORE any
	// decode, so the body never needs to be valid YAML — a giant comment is fine.
	const over = (1 << 20) + 1
	body := make([]byte, over)
	for i := range body {
		body[i] = '#'
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seeding oversize fixture: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(oversize) = nil error, want a size-cap failure")
	}

	var pe *pterr.Error
	if !errors.As(err, &pe) {
		t.Fatalf("error is not a *pterr.Error: %v", err)
	}
	if pe.Class != pterr.ClassConfig {
		t.Fatalf("error Class = %v, want ClassConfig", pe.Class)
	}
	if !strings.Contains(pe.Error(), "limit") {
		t.Fatalf("error = %q, want it to mention the size limit", pe.Error())
	}
}

func TestLoadMissingFileIsNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(missing) = nil error, want a not-exist error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("errors.Is(err, os.ErrNotExist) = false for %v, want true", err)
	}
}
