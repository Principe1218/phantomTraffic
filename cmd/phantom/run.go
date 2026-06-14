package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/engine"
	"github.com/Principe1218/phantomTraffic/internal/idgen"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// runRun loads, validates, and executes a phantom-traffic run.
//
// Exit codes:
//
//	0  — run completed or stopped cleanly (SIGINT/SIGTERM/--duration)
//	1  — run error (engine failed to start, or the run recorded an error)
//	2  — usage / IO error (bad flags, missing or invalid scenario file)
func runRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stderr, "Usage: phantom run [flags] <file>\n\nFlags:\n")
		fs.PrintDefaults()
	}

	allowInsecure := fs.Bool("allow-insecure", false,
		"permit blocks that declare allow_insecure (requires allow_insecure_reason per block)")
	capOverride := fs.Bool("i-understand-this-can-dos-targets", false,
		"permit declared caps to exceed the safety ceiling (operator opt-in only)")
	agentCount := fs.Int("agent-count", 1,
		"number of agents the aggregate caps are divided across (>= 1)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON output instead of human text")
	duration := fs.Duration("duration", 0,
		"maximum run time; 0 means run until every scenario block's duration elapses")

	if err := fs.Parse(args); err != nil {
		// flag already wrote the error + usage to stderr.
		return 2
	}
	file := fs.Arg(0)
	if file == "" {
		_, _ = io.WriteString(stderr, "phantom run: missing <file> argument\n")
		fs.Usage()
		return 2
	}
	if *agentCount < 1 {
		_, _ = fmt.Fprintf(stderr, "phantom run: --agent-count must be >= 1, got %d\n", *agentCount)
		return 2
	}

	raw, err := scenario.Load(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, _ = fmt.Fprintf(stderr, "phantom run: file not found: %s\n", file)
			return 2
		}
		_, _ = fmt.Fprintf(stderr, "phantom run: load %s: %v\n", file, err)
		return 2
	}

	scn, err := scenario.Validate(raw, scenario.Options{
		AllowInsecure: *allowInsecure,
		CapOverride:   *capOverride,
		AgentCount:    *agentCount,
	})
	if err != nil {
		var verrs pterr.FieldErrors
		if errors.As(err, &verrs) {
			for _, fe := range verrs {
				_, _ = fmt.Fprintf(stderr, "  %s: %s\n", fe.Field, fe.Msg)
			}
			_, _ = fmt.Fprintf(stderr, "✗ %s: invalid (%d errors)\n", file, len(verrs))
			return 1
		}
		_, _ = fmt.Fprintf(stderr, "phantom run: validate %s: %v\n", file, err)
		return 1
	}

	agentID, err := idgen.CorrelationID()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "phantom run: generate agent id: %v\n", err)
		return 1
	}

	reg := protocols.NewRegistry()
	if err := reg.Register(engine.NoopHandler{}); err != nil {
		_, _ = fmt.Fprintf(stderr, "phantom run: register handler: %v\n", err)
		return 1
	}

	// Seed the RNG from real entropy at startup; outside internal/engine so
	// the forbidigo time.Now boundary is not crossed inside the package.
	r := rng.New(
		uint64(time.Now().UnixNano()),
		uint64(uint(os.Getpid()))*0x9e3779b97f4a7c15,
	)

	e, err := engine.New(engine.Options{
		Clock:        clock.NewReal(),
		Rand:         r,
		Registry:     reg,
		SessionMaker: behavior.NewSessionMaker(),
		Logger:       slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Audit:        audit.NewNopSink(),
		AgentID:      agentID,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "phantom run: build engine: %v\n", err)
		return 1
	}

	// Always start the run with a plain background context so lifecycle
	// cancellation (Stop) goes through the Run.Stop path, preserving the state
	// machine transitions and drain semantics.
	run, err := e.Start(context.Background(), scn)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "phantom run: start: %v\n", err)
		return 1
	}

	if !*asJSON {
		_, _ = fmt.Fprintf(stdout, "▶ run %s started\n", run.ID())
	}

	// Build a deadline context for --duration; if unset, it never fires.
	limitCtx := context.Background()
	if *duration > 0 {
		var limitCancel context.CancelFunc
		limitCtx, limitCancel = context.WithTimeout(context.Background(), *duration)
		defer limitCancel()
	}

	// Signal handling: SIGINT / SIGTERM requests a graceful Stop.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	stopRun := func() {
		graceCtx, graceCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer graceCancel()
		_ = run.Stop(graceCtx)
	}

	select {
	case <-run.Wait():
		// Scenario duration elapsed: run completed naturally.
	case <-limitCtx.Done():
		// --duration timeout: graceful stop.
		stopRun()
	case <-sigCh:
		// Operator signal: graceful stop.
		stopRun()
	}

	snap := run.Snapshot()
	if runErr := run.Err(); runErr != nil {
		_, _ = fmt.Fprintf(stderr, "phantom run: %v\n", runErr)
		return 1
	}

	if *asJSON {
		writeRunReport(stdout, run.ID(), run.State().String(), snap)
	} else {
		_, _ = fmt.Fprintf(stdout,
			"✓ run %s %s: %d requests (%d ok, %d failed, %d panics)\n",
			run.ID(), run.State(), snap.Requests, snap.Successes, snap.Failures, snap.Panics)
	}
	return 0
}

func writeRunReport(w io.Writer, runID, state string, snap engine.StatsSnapshot) {
	doc := runReport{
		RunID:     runID,
		State:     state,
		Requests:  snap.Requests,
		Successes: snap.Successes,
		Failures:  snap.Failures,
		Panics:    snap.Panics,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		_, _ = io.WriteString(w, "{\"state\":\"error\"}\n")
		return
	}
	_, _ = w.Write(b)
	_, _ = io.WriteString(w, "\n")
}

// runReport is the JSON document emitted by phantom run --json.
type runReport struct {
	RunID     string `json:"run_id"`
	State     string `json:"state"`
	Requests  int64  `json:"requests"`
	Successes int64  `json:"successes"`
	Failures  int64  `json:"failures"`
	Panics    int64  `json:"panics"`
}
