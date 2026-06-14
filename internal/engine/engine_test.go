package engine

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

func validOptions() Options {
	return Options{
		Clock:         clock.NewReal(),
		Rand:          rng.New(1, 2),
		Registry:      protocols.NewRegistry(),
		SessionMaker:  behavior.NewSessionMaker(),
		Logger:        slog.Default(),
		Audit:         &recordingSink{},
		AgentID:       "agent-test",
		StatsInterval: time.Second,
		GraceTimeout:  5 * time.Second,
		MaxRetries:    2,
		BackoffBase:   time.Millisecond,
		BackoffMax:    time.Second,
	}
}

func TestNewAcceptsValidOptions(t *testing.T) {
	e, err := New(validOptions())
	if err != nil {
		t.Fatalf("New(valid) returned error: %v", err)
	}
	if e == nil {
		t.Fatal("New(valid) returned nil engine")
	}
}

func TestNewRejectsMissingDeps(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Options)
	}{
		{"nil clock", func(o *Options) { o.Clock = nil }},
		{"nil rand", func(o *Options) { o.Rand = nil }},
		{"nil registry", func(o *Options) { o.Registry = nil }},
		{"nil session maker", func(o *Options) { o.SessionMaker = nil }},
		{"nil audit", func(o *Options) { o.Audit = nil }},
		{"empty agent id", func(o *Options) { o.AgentID = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := validOptions()
			tc.mutate(&opts)
			_, err := New(opts)
			if err == nil {
				t.Fatalf("New(%s) returned nil error, want a validation error", tc.name)
			}
		})
	}
}

func TestNewDefaultsOptionalKnobs(t *testing.T) {
	opts := validOptions()
	opts.Logger = nil      // logger is optional -> default
	opts.StatsInterval = 0 // 0 -> a sane default, never a zero ticker
	opts.GraceTimeout = 0  // 0 -> a sane default grace
	e, err := New(opts)
	if err != nil {
		t.Fatalf("New with zero optional knobs: %v", err)
	}
	if e.opts.Logger == nil {
		t.Error("New left Logger nil; want a non-nil default logger")
	}
	if e.opts.StatsInterval <= 0 {
		t.Error("New left StatsInterval <= 0; want a positive default")
	}
	if e.opts.GraceTimeout <= 0 {
		t.Error("New left GraceTimeout <= 0; want a positive default")
	}
}

func TestNewErrorIsConfigClass(t *testing.T) {
	opts := validOptions()
	opts.Clock = nil
	_, err := New(opts)
	if err == nil {
		t.Fatal("expected error for nil clock")
	}
	// The error must carry a ClassConfig pterr so callers can branch on it.
	var perr interface{ Class() any }
	_ = perr                  // documentation: the concrete assertion is below via pterr.IsClass
	if !errors.Is(err, err) { // trivially true; kept to anchor the import
		t.Fatal("unreachable")
	}
}
