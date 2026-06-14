package safety

// CapPatch carries optional per-field overrides for a live caps re-tune. A nil
// field means "leave unchanged". Lowering a cap is always free; raising one is
// gated by the run's audited cap-override flag (enforced in the engine, not
// here).
type CapPatch struct {
	PerTargetRPS       *float64
	GlobalRPS          *float64
	TotalRequestBudget *int64
}

// WithPatch returns a copy of declared with every non-nil CapPatch field
// overlaid. The receiver is not mutated.
func (declared CapSpec) WithPatch(p CapPatch) CapSpec {
	out := declared
	if p.PerTargetRPS != nil {
		out.PerTargetRPS = *p.PerTargetRPS
	}
	if p.GlobalRPS != nil {
		out.GlobalRPS = *p.GlobalRPS
	}
	if p.TotalRequestBudget != nil {
		out.TotalRequestBudget = *p.TotalRequestBudget
	}
	return out
}
