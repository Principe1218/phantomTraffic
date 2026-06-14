// Package persona is the YAML data layer for PhantomTraffic personas. It decodes
// and validates built-in and custom personas into internal/behavior types and
// resolves them by name. It imports internal/behavior and is NEVER imported by it
// (cycle-free). Like internal/behavior it is inside the forbidigo/math-rand lint
// boundary: no time.Now/time.Sleep, no math/rand.
package persona

import "github.com/Principe1218/phantomTraffic/internal/behavior"

// Persona is a validated bundle of realism primitives plus a protocol mix and
// session shape. It is produced only by Compile/Lookup and is immutable.
type Persona struct {
	Name      string
	Mix       behavior.TemplateMix
	ThinkTime behavior.Distribution
	Jitter    behavior.JitterModel
	Burst     behavior.BurstModel
	TimeOfDay behavior.TimeOfDayShaper
	Prints    behavior.FingerprintPool
	Shape     behavior.SessionShape
	Bounds    behavior.BranchBounds
}

// ToSpec decomposes the persona into a behavior.SessionSpec for the factory. The
// caller supplies the TargetSelector (which encodes the scenario's targets +
// rotation policy — the engine in Plan 4, or a behavior.RoundRobinSelector in
// tests).
func (p Persona) ToSpec(selector behavior.TargetSelector) behavior.SessionSpec {
	return behavior.SessionSpec{
		Mix:       p.Mix,
		ThinkTime: p.ThinkTime,
		Jitter:    p.Jitter,
		Burst:     p.Burst,
		TimeOfDay: p.TimeOfDay,
		Prints:    p.Prints,
		Shape:     p.Shape,
		Bounds:    p.Bounds,
		Selector:  selector,
	}
}
