package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// runYAML is a valid scenario with an inline persona for phantom run tests. The
// persona uses constant distributions so the behavior is deterministic and fast.
// A duration_minutes of 5 is used; tests apply --duration to stop the run early.
const runYAML = `name: run-test
description: minimal scenario for phantom run CLI tests
allowed_domains:
  - example.com
scenarios:
  - id: web
    protocol: http
    targets:
      - example.com:443
    concurrency: 1
    duration_minutes: 5
    persona: noop-persona
    weight: 1
personas:
  - name: noop-persona
    think_time:
      kind: constant
      d: 10ms
    jitter:
      kind: none
    burst:
      active:
        kind: constant
        d: 1h
      idle:
        kind: constant
        d: 1ms
    session:
      length:
        kind: constant
        d: 1h
      abandon: 0
    mix:
      - protocol: http
        verb: fetch-page
        cause: navigation
        pacing: shaper-managed
        weight: 1
    fingerprints: default
`

func TestRunRun_MissingPositionalArg_ExitCode2(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runRun([]string{"--json"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "missing <file>") {
		t.Fatalf("stderr = %q, want missing-file message", errBuf.String())
	}
}

func TestRunRun_MissingFile_ExitCode2(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	var out, errBuf bytes.Buffer
	code := runRun([]string{missing}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "not found") {
		t.Fatalf("stderr = %q, want 'not found' message", errBuf.String())
	}
}

func TestRunRun_InvalidScenario_ExitCode1(t *testing.T) {
	path := writeScenario(t, invalidYAML)
	var out, errBuf bytes.Buffer
	code := runRun([]string{path}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (invalid scenario); stderr=%q", code, errBuf.String())
	}
}

func TestRunRun_AgentCountZero_ExitCode2(t *testing.T) {
	path := writeScenario(t, runYAML)
	var out, errBuf bytes.Buffer
	code := runRun([]string{"--agent-count", "0", path}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (bad --agent-count); stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "agent-count") {
		t.Fatalf("stderr = %q, want agent-count error", errBuf.String())
	}
}

// TestRunRun_DurationStop_ExitCode0 verifies that a valid run started with
// --duration exits cleanly (code 0) when the duration elapses before the
// scenario's own duration_minutes. Workers start with real-clock sleeps;
// --duration cancels them via Stop after the timeout.
func TestRunRun_DurationStop_ExitCode0(t *testing.T) {
	path := writeScenario(t, runYAML)
	var out, errBuf bytes.Buffer
	code := runRun([]string{"--duration", "200ms", path}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, errBuf.String(), out.String())
	}
	if !strings.Contains(out.String(), "run") {
		t.Fatalf("stdout = %q, want a run summary line", out.String())
	}
}

// TestRunRun_JSON_DurationStop_ExitCode0 verifies that --json emits a valid
// JSON document with the expected fields after a --duration-triggered stop.
func TestRunRun_JSON_DurationStop_ExitCode0(t *testing.T) {
	path := writeScenario(t, runYAML)
	var out, errBuf bytes.Buffer
	code := runRun([]string{"--json", "--duration", "200ms", path}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	var rep runReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out.String())
	}
	if rep.RunID == "" {
		t.Fatalf("run_id missing from JSON: %s", out.String())
	}
	if rep.State == "" {
		t.Fatalf("state missing from JSON: %s", out.String())
	}
}

// TestDispatch_Run_RoutesToRunRun verifies that dispatch routes the "run"
// subcommand. Passing --duration avoids waiting for the scenario duration.
func TestDispatch_Run_RoutesToRunRun(t *testing.T) {
	path := writeScenario(t, runYAML)
	var out, errBuf bytes.Buffer
	code := dispatch([]string{"run", "--duration", "200ms", path}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("dispatch(run) = %d, want 0; stderr=%q", code, errBuf.String())
	}
}
