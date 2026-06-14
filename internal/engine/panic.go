package engine

import "log/slog"

// runGuarded runs fn inside a recover() shim and reports whether it panicked. No
// handler ever runs on a bare go statement (design §7.2): a panic becomes a
// counted, recoverable event rather than a process crash.
//
// Redaction (AGENTS.md §5.5): the recovered value is logged at Error as a single
// structured field, and the raw goroutine stack is deliberately NOT included so a
// panic message cannot leak sensitive frame data into a log or the UI. The caller
// turns panicked=true into an OutcomePanicked Result.
func runGuarded(log *slog.Logger, fn func()) (panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
			// Log only a redacted marker; never the raw stack (debug.Stack()).
			log.Error("worker recovered from panic", slog.String("panic", "recovered"))
		}
	}()
	fn()
	return false
}
