package behavior

import "testing"

func TestBranchBoundsAllows(t *testing.T) {
	b := DefaultBranchBounds()
	if !b.Allows(BranchRedirect, b.MaxRedirects-1) {
		t.Fatal("one below the redirect bound must be allowed")
	}
	if b.Allows(BranchRedirect, b.MaxRedirects) {
		t.Fatal("at the redirect bound must be denied")
	}
	if b.Allows(BranchKind(99), 0) {
		t.Fatal("unknown branch kind must be denied")
	}
}
