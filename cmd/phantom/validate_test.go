package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeScenario writes content to a temp file and returns its path.
func writeScenario(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "scenario.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G304 -- test fixture path under t.TempDir()
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// validYAML is a minimal, fully-valid scenario: one block, one target, no
// insecure flags, no caps over ceiling. It must validate with default Options.
const validYAML = `name: smoke
description: a minimal valid scenario
allowed_domains:
  - example.com
execution:
  mode: parallel
  stop_on_error: false
scenarios:
  - id: web-1
    protocol: http
    targets:
      - example.com:443
    target_rotation: sequential
`

func TestRunValidate_HappyPath_Human(t *testing.T) {
	path := writeScenario(t, validYAML)

	var out, errBuf bytes.Buffer
	code := runValidate([]string{path}, &out, &errBuf)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "valid") {
		t.Fatalf("stdout = %q, want to contain %q", got, "valid")
	}
	if !strings.Contains(got, "1 scenarios") {
		t.Fatalf("stdout = %q, want scenario count %q", got, "1 scenarios")
	}
	if !strings.Contains(got, "1 targets") {
		t.Fatalf("stdout = %q, want target count %q", got, "1 targets")
	}
}

func TestRunValidate_FlagsPrecedeFile(t *testing.T) {
	path := writeScenario(t, validYAML)

	// stdlib flag requires flags before the positional file.
	var out, errBuf bytes.Buffer
	code := runValidate([]string{"--agent-count", "2", path}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "valid") {
		t.Fatalf("stdout = %q, want to contain %q", out.String(), "valid")
	}
}

// invalidYAML decodes fine (all keys known) but fails validation: empty name
// and an unknown protocol "gopher". scenario.Validate aggregates BOTH.
const invalidYAML = `name: ""
description: invalid on purpose
allowed_domains:
  - example.com
execution:
  mode: parallel
scenarios:
  - id: bad-1
    protocol: gopher
    targets:
      - example.com:70
`

func TestRunValidate_Invalid_HumanListsFieldErrors(t *testing.T) {
	path := writeScenario(t, invalidYAML)

	var out, errBuf bytes.Buffer
	code := runValidate([]string{path}, &out, &errBuf)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on invalid (human mode)", out.String())
	}
	got := errBuf.String()
	// Each field error is rendered "  <field>: <msg>".
	if !strings.Contains(got, "name") {
		t.Fatalf("stderr = %q, want a 'name' field error", got)
	}
	if !strings.Contains(got, "protocol") {
		t.Fatalf("stderr = %q, want a 'protocol' field error", got)
	}
	// Trailing summary with a count.
	if !strings.Contains(got, "invalid (") {
		t.Fatalf("stderr = %q, want trailing 'invalid (<n> errors)' summary", got)
	}
	// Lines for individual errors are indented two spaces.
	if !strings.HasPrefix(got, "  ") && !strings.Contains(got, "\n  ") {
		t.Fatalf("stderr = %q, want indented per-field error lines", got)
	}
}

// jsonResult mirrors the validateReport JSON shape for decoding in tests.
type jsonResult struct {
	Valid  bool `json:"valid"`
	Errors []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
		Class   string `json:"class"`
	} `json:"errors"`
	Summary *struct {
		Name      string `json:"name"`
		Scenarios int    `json:"scenarios"`
		Targets   int    `json:"targets"`
	} `json:"summary"`
}

func TestRunValidate_JSON_Valid(t *testing.T) {
	path := writeScenario(t, validYAML)

	var out, errBuf bytes.Buffer
	code := runValidate([]string{"--json", path}, &out, &errBuf)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on valid", errBuf.String())
	}
	var res jsonResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out.String())
	}
	if !res.Valid {
		t.Fatalf("res.Valid = false, want true; %s", out.String())
	}
	if len(res.Errors) != 0 {
		t.Fatalf("res.Errors = %v, want empty on valid", res.Errors)
	}
	if res.Summary == nil {
		t.Fatalf("res.Summary = nil, want a summary block; %s", out.String())
	}
	if res.Summary.Name != "smoke" {
		t.Fatalf("summary.name = %q, want %q", res.Summary.Name, "smoke")
	}
	if res.Summary.Scenarios != 1 {
		t.Fatalf("summary.scenarios = %d, want 1", res.Summary.Scenarios)
	}
	if res.Summary.Targets != 1 {
		t.Fatalf("summary.targets = %d, want 1", res.Summary.Targets)
	}
}

func TestRunValidate_JSON_Invalid(t *testing.T) {
	path := writeScenario(t, invalidYAML)

	var out, errBuf bytes.Buffer
	code := runValidate([]string{"--json", path}, &out, &errBuf)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, errBuf.String())
	}
	var res jsonResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out.String())
	}
	if res.Valid {
		t.Fatalf("res.Valid = true, want false; %s", out.String())
	}
	if len(res.Errors) == 0 {
		t.Fatalf("res.Errors empty, want >= 1 error; %s", out.String())
	}
	if res.Summary != nil {
		t.Fatalf("res.Summary = %v, want nil on invalid", res.Summary)
	}
	// Every error carries a field, a message, and the config class.
	for i, e := range res.Errors {
		if e.Field == "" {
			t.Fatalf("errors[%d].field empty; %s", i, out.String())
		}
		if e.Message == "" {
			t.Fatalf("errors[%d].message empty; %s", i, out.String())
		}
		if e.Class != "config" {
			t.Fatalf("errors[%d].class = %q, want %q", i, e.Class, "config")
		}
	}
}

func TestRunValidate_MissingFile_ExitCode2(t *testing.T) {
	// A path inside a fresh temp dir that we never create.
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	var out, errBuf bytes.Buffer
	code := runValidate([]string{missing}, &out, &errBuf)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (IO error); stderr=%q", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on missing file", out.String())
	}
	if !strings.Contains(errBuf.String(), "not found") {
		t.Fatalf("stderr = %q, want a 'not found' message", errBuf.String())
	}
}

func TestRunValidate_MissingPositionalArg_ExitCode2(t *testing.T) {
	// No file argument at all is a usage error (code 2), distinct from a
	// missing-on-disk file.
	var out, errBuf bytes.Buffer
	code := runValidate([]string{"--json"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error); stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "missing <file>") {
		t.Fatalf("stderr = %q, want a missing-file usage message", errBuf.String())
	}
}

func TestRunValidate_AgentCountZero_ExitCode2(t *testing.T) {
	path := writeScenario(t, validYAML)
	var out, errBuf bytes.Buffer
	code := runValidate([]string{"--agent-count", "0", path}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (bad --agent-count); stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "agent-count") {
		t.Fatalf("stderr = %q, want an agent-count usage message", errBuf.String())
	}
}

// insecureYAML declares a block that requires the insecure invocation flag:
// allow_insecure with a non-empty reason. Per D4, scenario.Validate rejects it
// unless Options.AllowInsecure (the --allow-insecure flag) is true. The
// protocol is a KNOWN one so the insecure gate is the sole failure cause.
const insecureYAML = `name: insecure-probe
description: needs the operator to opt into insecure transport
allowed_domains:
  - internal.test
execution:
  mode: sequential
  stop_on_error: true
scenarios:
  - id: legacy-1
    protocol: http
    targets:
      - internal.test:8443
    allow_insecure: true
    allow_insecure_reason: "legacy appliance has a self-signed cert, lab only"
`

func TestRunValidate_InsecureGate_RejectedWithoutFlag(t *testing.T) {
	path := writeScenario(t, insecureYAML)

	var out, errBuf bytes.Buffer
	code := runValidate([]string{path}, &out, &errBuf)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (insecure without flag); stderr=%q", code, errBuf.String())
	}
	got := errBuf.String()
	if !strings.Contains(got, "allow_insecure") && !strings.Contains(got, "allow-insecure") {
		t.Fatalf("stderr = %q, want an insecure-flag field error", got)
	}
}

func TestRunValidate_InsecureGate_AcceptedWithFlag(t *testing.T) {
	path := writeScenario(t, insecureYAML)

	var out, errBuf bytes.Buffer
	code := runValidate([]string{"--allow-insecure", path}, &out, &errBuf)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (insecure with flag); stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "valid") {
		t.Fatalf("stdout = %q, want a valid summary", out.String())
	}
}
