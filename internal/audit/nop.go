package audit

// nopSink is a Sink that accepts every event and silently discards it.
type nopSink struct{}

// NewNopSink returns a Sink that accepts every event and discards it without
// writing to disk. Append, Verify, and Close are all no-ops and always return
// nil. Safe for concurrent use. Intended for CLI smoke runs and integration tests
// where audit durability is not required.
func NewNopSink() Sink { return nopSink{} }

func (nopSink) Append(_ Event) error { return nil }
func (nopSink) Verify() error        { return nil }
func (nopSink) Close() error         { return nil }
