// Package scenario loads and validates a PhantomTraffic scenario file into an
// immutable, FROZEN Scenario. Validation is a PURE function (no network, no
// filesystem beyond the caller-supplied path at Load time, no runtime limiter,
// no audit writes). Every validation failure classifies as pterr.ClassConfig
// and is aggregated so a single Validate pass reports ALL problems at once
// (design §5.2, AGENTS.md §5.5: fail fast, fully, at config time).
package scenario

import (
	"time"

	"github.com/Principe1218/phantomTraffic/internal/persona"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// Block is one validated, FROZEN scenario block: a protocol, its typed targets,
// a rotation policy, and the D4 insecure gate. It is produced only by Validate
// and MUST NOT be mutated afterward (the engine treats it as immutable input).
type Block struct {
	ID                  string
	Protocol            protocols.ProtocolID
	Targets             []protocols.Target
	Rotation            RotationStrategy
	RotationInterval    time.Duration
	AllowInsecure       bool
	AllowInsecureReason string
	Persona             persona.Persona // resolved + frozen at Validate (Plan 3)
	Concurrency         int             // vusers for this block; defaults to 1, bounded by the effective cap
	Duration            time.Duration   // cumulative ACTIVE-traffic duration for this block
	Weight              uint            // relative weight under the scenario WeightBasis
	Ramp                RampPlan        // optional concurrency ramp-up; zero value => no ramp
}

// RampPlan is a FROZEN concurrency ramp-up for one block. Up is the window over
// which the block climbs from StartConcurrency to Block.Concurrency; a zero Up
// means start at full concurrency immediately. Ramp-DOWN is omitted in the MVP —
// a Stop drains naturally (design §6.6). Produced only by Validate; immutable.
type RampPlan struct {
	Up               time.Duration // 0 => instant (start at full concurrency)
	StartConcurrency int           // floor at run start; >= 1, <= Block.Concurrency
}

// ScheduleWindow is one FROZEN on/off window. Days is indexed by time.Weekday
// (Sunday == 0 .. Saturday == 6). Start and End are durations since local
// midnight in the parent Schedule.Loc; Validate guarantees End > Start and
// rejects cross-midnight windows, so no window ever wraps past midnight.
type ScheduleWindow struct {
	Days  [7]bool
	Start time.Duration // since-midnight, in Schedule.Loc
	End   time.Duration // since-midnight; End > Start
}

// Schedule is the FROZEN scenario-level on/off schedule. An empty Windows slice
// means "always active" (no scheduler goroutine is spawned). Windows evaluate in
// Loc. Produced only by Validate; immutable.
type Schedule struct {
	Loc     *time.Location
	Windows []ScheduleWindow
}

// Execution is the validated, FROZEN run-mode for the scenario as a whole.
type Execution struct {
	Mode        ExecutionMode
	StopOnError bool
}

// Scenario is the FROZEN, validated output of Validate: typed protocols,
// pre-parsed targets, the authoritative TargetSet allowlist, and effective caps.
// It is immutable after Validate returns — DO NOT MUTATE it or its TargetSet.
// Holding a Scenario is proof the config boundary held (design §5).
type Scenario struct {
	Name           string
	Description    string
	AllowedDomains []string
	AgentCount     int
	Caps           safety.CapSpec
	Ceiling        safety.Ceiling
	Execution      Execution
	Blocks         []Block
	Targets        protocols.TargetSet
	WeightBasis    WeightBasis // how block weights translate to load; default WeightByVuserPopulation
	Schedule       Schedule    // optional on/off windows; empty Windows => always active
}
