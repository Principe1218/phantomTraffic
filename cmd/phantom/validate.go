package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// runValidate parses validate flags + a positional scenario file, loads and
// validates it (purely; no network/audit/credential I/O), and renders the
// result. Return codes: 0 valid, 1 invalid scenario, 2 usage/IO error.
func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stderr, "Usage: phantom validate [flags] <file>\n\nFlags:\n")
		fs.PrintDefaults()
	}

	allowInsecure := fs.Bool("allow-insecure", false,
		"permit blocks that declare allow_insecure (requires a reason per block)")
	capOverride := fs.Bool("i-understand-this-can-dos-targets", false,
		"permit declared caps to exceed the safety ceiling")
	agentCount := fs.Int("agent-count", 1,
		"number of agents the aggregate caps are divided across (>= 1)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of human text")

	if err := fs.Parse(args); err != nil {
		// flag already wrote the parse error + usage to stderr (ContinueOnError).
		return 2
	}

	file := fs.Arg(0)
	if file == "" {
		_, _ = io.WriteString(stderr, "phantom validate: missing <file> argument\n")
		fs.Usage()
		return 2
	}
	if *agentCount < 1 {
		_, _ = fmt.Fprintf(stderr, "phantom validate: --agent-count must be >= 1, got %d\n", *agentCount)
		return 2
	}

	raw, err := scenario.Load(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, _ = fmt.Fprintf(stderr, "phantom validate: file not found: %s\n", file)
			return 2
		}
		// A decode/size error is an invalid scenario (code 1), not an IO error.
		return renderDecodeError(file, err, *asJSON, stdout, stderr)
	}

	scn, err := scenario.Validate(raw, scenario.Options{
		AllowInsecure: *allowInsecure,
		CapOverride:   *capOverride,
		AgentCount:    *agentCount,
	})
	if err != nil {
		var verrs pterr.FieldErrors
		if errors.As(err, &verrs) {
			return renderValidationErrors(file, verrs, *asJSON, stdout, stderr)
		}
		// Defensive: any non-ValidationErrors error from a pure validator is
		// still an invalid scenario; render its message without internals.
		return renderDecodeError(file, err, *asJSON, stdout, stderr)
	}

	return renderSuccess(file, scn, *asJSON, stdout)
}

// countTargets returns the total number of targets in the frozen TargetSet
// across every known protocol.
func countTargets(ts protocols.TargetSet) int {
	total := 0
	for _, p := range protocols.KnownProtocols() {
		total += len(ts.TargetsFor(p))
	}
	return total
}

// renderSuccess prints the valid-scenario summary and returns 0.
func renderSuccess(file string, scn scenario.Scenario, asJSON bool, stdout io.Writer) int {
	nScenarios := len(scn.Blocks)
	nTargets := countTargets(scn.Targets)
	if asJSON {
		writeJSON(stdout, validateReport{
			Valid: true,
			Summary: &summaryReport{
				Name:      scn.Name,
				Scenarios: nScenarios,
				Targets:   nTargets,
			},
		})
		return 0
	}
	_, _ = fmt.Fprintf(stdout, "✓ %s: valid — %d scenarios, %d targets\n",
		file, nScenarios, nTargets)
	return 0
}

// renderValidationErrors lists each field error on stderr (human) or emits a
// JSON document on stdout, and returns exit code 1. Messages are the
// redaction-safe FieldError.Msg values (AGENTS.md §5.5) — no internals.
func renderValidationErrors(file string, verrs pterr.FieldErrors, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		reps := make([]fieldReport, 0, len(verrs))
		for _, fe := range verrs {
			reps = append(reps, fieldReport{
				Field:   fe.Field,
				Message: fe.Msg,
				Class:   fe.Class().String(),
			})
		}
		writeJSON(stdout, validateReport{Valid: false, Errors: reps})
		return 1
	}
	for _, fe := range verrs {
		_, _ = fmt.Fprintf(stderr, "  %s: %s\n", fe.Field, fe.Msg)
	}
	_, _ = fmt.Fprintf(stderr, "✗ %s: invalid (%d errors)\n", file, len(verrs))
	return 1
}

// renderDecodeError renders a single load/decode error as an invalid scenario
// (exit 1). The error text comes from scenario.Load's *pterr.Error (ClassConfig)
// and is redaction-safe.
func renderDecodeError(file string, err error, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		writeJSON(stdout, validateReport{
			Valid: false,
			Errors: []fieldReport{{
				Field:   "<file>",
				Message: err.Error(),
				Class:   "config",
			}},
		})
		return 1
	}
	_, _ = fmt.Fprintf(stderr, "  %s: %s\n", file, err.Error())
	_, _ = fmt.Fprintf(stderr, "✗ %s: invalid (1 errors)\n", file)
	return 1
}

// writeJSON marshals the report with two-space indentation and writes it,
// newline-terminated, to w. It never panics: a marshal failure (unreachable
// for this plain struct) degrades to a minimal valid document.
func writeJSON(w io.Writer, v validateReport) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		_, _ = io.WriteString(w, "{\"valid\":false}\n")
		return
	}
	_, _ = w.Write(b)
	_, _ = io.WriteString(w, "\n")
}

// validateReport is the top-level JSON document emitted with --json.
type validateReport struct {
	Valid   bool           `json:"valid"`
	Errors  []fieldReport  `json:"errors,omitempty"`
	Summary *summaryReport `json:"summary,omitempty"`
}

// fieldReport is one field-scoped validation error in JSON output.
type fieldReport struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Class   string `json:"class"`
}

// summaryReport is the success summary block in JSON output.
type summaryReport struct {
	Name      string `json:"name"`
	Scenarios int    `json:"scenarios"`
	Targets   int    `json:"targets"`
}
