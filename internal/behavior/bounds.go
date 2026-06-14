package behavior

// BranchKind names a bounded stateful-branching loop in the behavior layer.
type BranchKind uint8

const (
	BranchRedirect BranchKind = iota
	BranchCNAME
	BranchLinkFollow
	BranchABRSwitch
)

// BranchBounds caps the Observe-driven branching loops so a closed-loop reaction
// can never defeat safety (design §4). The bounds are carried by the Session and
// enforced by the handler plans (5–8) that own the Observation-driven branches;
// Plan 3 ships the pure Allows primitive (the branching itself is an inert seam
// until those plans land).
type BranchBounds struct {
	MaxRedirects     int
	MaxCNAME         int
	MaxLinksFollowed int
	MaxABRSwitches   int
}

// DefaultBranchBounds returns conservative caps for human-scale branching.
func DefaultBranchBounds() BranchBounds {
	return BranchBounds{MaxRedirects: 10, MaxCNAME: 8, MaxLinksFollowed: 20, MaxABRSwitches: 12}
}

// Allows reports whether one more branch of kind is within bounds, given how many
// of that kind have already occurred.
func (b BranchBounds) Allows(kind BranchKind, count int) bool {
	switch kind {
	case BranchRedirect:
		return count < b.MaxRedirects
	case BranchCNAME:
		return count < b.MaxCNAME
	case BranchLinkFollow:
		return count < b.MaxLinksFollowed
	case BranchABRSwitch:
		return count < b.MaxABRSwitches
	default:
		return false
	}
}
