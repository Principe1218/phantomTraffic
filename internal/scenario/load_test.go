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

	if raw.Name != "smoke" {
		t.Errorf("Name = %q, want %q", raw.Name, "smoke")
	}
	if raw.Description != "minimal valid scenario for decode tests" {
		t.Errorf("Description = %q, want the fixture description", raw.Description)
	}
	if got, want := len(raw.AllowedDomains), 2; got != want {
		t.Fatalf("len(AllowedDomains) = %d, want %d", got, want)
	}
	if raw.AllowedDomains[0] != "example.com" || raw.AllowedDomains[1] != "api.example.com" {
		t.Errorf("AllowedDomains = %v, want [example.com api.example.com]", raw.AllowedDomains)
	}

	// Caps decode into the typed fields with the exact yaml tags.
	if raw.Caps.PerTargetRPS != 5 {
		t.Errorf("Caps.PerTargetRPS = %v, want 5", raw.Caps.PerTargetRPS)
	}
	if raw.Caps.GlobalRPS != 25 {
		t.Errorf("Caps.GlobalRPS = %v, want 25", raw.Caps.GlobalRPS)
	}
	if raw.Caps.MaxConcurrentSessions != 10 {
		t.Errorf("Caps.MaxConcurrentSessions = %d, want 10", raw.Caps.MaxConcurrentSessions)
	}
	if raw.Caps.TotalRequestBudget != 100000 {
		t.Errorf("Caps.TotalRequestBudget = %d, want 100000", raw.Caps.TotalRequestBudget)
	}
	if raw.Caps.StreamingByteRateKbps != 6000 {
		t.Errorf("Caps.StreamingByteRateKbps = %d, want 6000", raw.Caps.StreamingByteRateKbps)
	}
	if raw.Caps.ConcurrentStreams != 2 {
		t.Errorf("Caps.ConcurrentStreams = %d, want 2", raw.Caps.ConcurrentStreams)
	}
	if raw.Caps.PerSessionMaxDurationSeconds != 600 {
		t.Errorf("Caps.PerSessionMaxDurationSeconds = %d, want 600", raw.Caps.PerSessionMaxDurationSeconds)
	}
	if raw.Caps.PerSessionMaxActions != 5000 {
		t.Errorf("Caps.PerSessionMaxActions = %d, want 5000", raw.Caps.PerSessionMaxActions)
	}

	// Execution.
	if raw.Execution.Mode != "sequential" {
		t.Errorf("Execution.Mode = %q, want sequential", raw.Execution.Mode)
	}
	if !raw.Execution.StopOnError {
		t.Errorf("Execution.StopOnError = false, want true")
	}

	// Scenario blocks.
	if got, want := len(raw.Scenarios), 2; got != want {
		t.Fatalf("len(Scenarios) = %d, want %d", got, want)
	}
	first := raw.Scenarios[0]
	if first.ID != "web-browse" {
		t.Errorf("Scenarios[0].ID = %q, want web-browse", first.ID)
	}
	if first.Protocol != "http" {
		t.Errorf("Scenarios[0].Protocol = %q, want http", first.Protocol)
	}
	if got, want := len(first.Targets), 2; got != want {
		t.Fatalf("len(Scenarios[0].Targets) = %d, want %d", got, want)
	}
	if first.Targets[0] != "example.com:443" {
		t.Errorf("Scenarios[0].Targets[0] = %q, want example.com:443", first.Targets[0])
	}
	if first.TargetRotation != "random" {
		t.Errorf("Scenarios[0].TargetRotation = %q, want random", first.TargetRotation)
	}
	if first.TargetRotationIntervalSeconds != 30 {
		t.Errorf("Scenarios[0].TargetRotationIntervalSeconds = %d, want 30", first.TargetRotationIntervalSeconds)
	}
	if first.AllowInsecure {
		t.Errorf("Scenarios[0].AllowInsecure = true, want false")
	}

	second := raw.Scenarios[1]
	if second.ID != "name-lookups" || second.Protocol != "dns" {
		t.Errorf("Scenarios[1] = {ID:%q Protocol:%q}, want {name-lookups dns}", second.ID, second.Protocol)
	}
	if second.TargetRotationIntervalSeconds != 0 {
		t.Errorf("Scenarios[1].TargetRotationIntervalSeconds = %d, want 0", second.TargetRotationIntervalSeconds)
	}
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
