package engine

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain wraps every test in this package with goleak's leak detector, asserting
// zero residual goroutines after the package's tests complete. Clean teardown is the
// determinism guarantee the whole engine is built around (design §6.3, §7.3).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
