package behavior

import (
	"fmt"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// Template is one weighted action choice in a persona's mix: which protocol and
// verb, and the Cause/Pacing that drive timing. It carries NO concrete params —
// those come from the scenario's per-protocol action sources in handler plans
// (5–8). The Session emits it as a PlannedAction with Params == nil.
type Template struct {
	Protocol protocols.ProtocolID
	Verb     protocols.ActionKind
	Cause    protocols.Cause
	Pacing   protocols.PacingMode
	Weight   float64
}

// Ref returns the protocol-agnostic action reference for this template.
func (t Template) Ref() protocols.Ref {
	return protocols.Ref{Protocol: t.Protocol, Verb: t.Verb}
}

// TemplateMix is a weighted set of Templates. Pick draws one proportional to its
// weight with a single Float64 draw, so a scripted rng makes selection
// deterministic. Construct it only via NewTemplateMix.
type TemplateMix struct {
	templates  []Template
	cumWeights []float64 // cumulative weights; last element == total
	total      float64
}

// NewTemplateMix builds a mix and validates weights. The set must be non-empty
// and every weight must be > 0, so a template can never be silently unreachable.
func NewTemplateMix(templates []Template) (TemplateMix, error) {
	if len(templates) == 0 {
		return TemplateMix{}, fmt.Errorf("behavior: template mix must be non-empty")
	}
	cum := make([]float64, len(templates))
	var total float64
	for i, t := range templates {
		if t.Weight <= 0 {
			return TemplateMix{}, fmt.Errorf("behavior: template[%d] (%s) weight must be > 0", i, t.Ref())
		}
		total += t.Weight
		cum[i] = total
	}
	return TemplateMix{templates: append([]Template(nil), templates...), cumWeights: cum, total: total}, nil
}

// Pick draws a template proportional to weight.
func (m TemplateMix) Pick(r rng.Rand) Template {
	x := r.Float64() * m.total
	for i, c := range m.cumWeights {
		if x < c {
			return m.templates[i]
		}
	}
	return m.templates[len(m.templates)-1] // float edge case: x == total
}

// Len reports the number of templates.
func (m TemplateMix) Len() int { return len(m.templates) }

// SessionShape governs a vuser's lifecycle: total Length (sampled once at
// construction), the InterTask gap distribution, and a per-step Abandon
// probability ∈ [0,1] for early exit.
type SessionShape struct {
	Length    Distribution
	InterTask Distribution
	Abandon   float64
}
