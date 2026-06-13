package scenario

import (
	"time"

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
}
