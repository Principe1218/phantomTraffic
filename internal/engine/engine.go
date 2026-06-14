package engine

import (
	"log/slog"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// Default tuning for optional Options knobs left at their zero value.
const (
	defaultStatsInterval = time.Second
	defaultGraceTimeout  = 30 * time.Second
	engineOptions        = "engine.options"
)

// Options carries every dependency Engine needs. Required deps are validated by
// New; optional knobs (Logger, StatsInterval, GraceTimeout) default when zero.
type Options struct {
	Clock        clock.Clock
	Rand         rng.Rand
	Registry     *protocols.Registry
	SessionMaker behavior.SessionMaker
	Logger       *slog.Logger
	Audit        audit.Sink
	AgentID      string

	StatsInterval time.Duration
	GraceTimeout  time.Duration

	MaxRetries  int
	BackoffBase time.Duration
	BackoffMax  time.Duration
}

// Engine is the transport-agnostic run factory. UI and CLI both call New + Start.
type Engine struct {
	opts Options
}

// New validates the required dependencies and fills sane defaults for optional
// knobs. A missing required dep is a ClassConfig error (fail fast, never start).
func New(opts Options) (*Engine, error) {
	const op = "engine.New"
	switch {
	case opts.Clock == nil:
		return nil, pterr.New(pterr.ClassConfig, engineOptions, op, "Clock is required")
	case opts.Rand == nil:
		return nil, pterr.New(pterr.ClassConfig, engineOptions, op, "Rand is required")
	case opts.Registry == nil:
		return nil, pterr.New(pterr.ClassConfig, engineOptions, op, "Registry is required")
	case opts.SessionMaker == nil:
		return nil, pterr.New(pterr.ClassConfig, engineOptions, op, "SessionMaker is required")
	case opts.Audit == nil:
		return nil, pterr.New(pterr.ClassConfig, engineOptions, op, "Audit is required")
	case opts.AgentID == "":
		return nil, pterr.New(pterr.ClassConfig, engineOptions, op, "AgentID is required")
	}

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.StatsInterval <= 0 {
		opts.StatsInterval = defaultStatsInterval
	}
	if opts.GraceTimeout <= 0 {
		opts.GraceTimeout = defaultGraceTimeout
	}

	return &Engine{opts: opts}, nil
}
