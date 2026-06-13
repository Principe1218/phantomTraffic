package audit_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// fakeClock is a minimal clock.Clock for audit tests. Only Now() is exercised;
// the rest satisfy the interface. Time advances only when Set/Advance is called.
type fakeClock struct{ now time.Time }

func newFakeClock(t time.Time) *fakeClock            { return &fakeClock{now: t} }
func (c *fakeClock) Now() time.Time                  { return c.now }
func (c *fakeClock) Since(t time.Time) time.Duration { return c.now.Sub(t) }
func (c *fakeClock) Advance(d time.Duration)         { c.now = c.now.Add(d) }
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return nil
}
func (c *fakeClock) NewTimer(d time.Duration) clock.Timer            { return nil }
func (c *fakeClock) AfterFunc(d time.Duration, f func()) clock.Timer { return nil }

var _ clock.Clock = (*fakeClock)(nil)

func TestActionVocabulary(t *testing.T) {
	cases := map[audit.Action]string{
		audit.ActionTLSVerificationSkipped: "tls.verification_skipped",
		audit.ActionSSHHostKeyUnverified:   "ssh.host_key_unverified",
		audit.ActionCapOverrideEnabled:     "safety.cap_override_enabled",
		audit.ActionScenarioStarted:        "scenario.started",
		audit.ActionScenarioStopped:        "scenario.stopped",
		audit.ActionScenarioPatched:        "scenario.patched",
	}
	for action, want := range cases {
		if string(action) != want {
			t.Errorf("action %q = %q, want %q", action, string(action), want)
		}
	}
}

func TestValidateEventRejectsEmptyFields(t *testing.T) {
	tests := []struct {
		name  string
		event audit.Event
	}{
		{"empty actor", audit.Event{Actor: "", Action: audit.ActionScenarioStarted, Resource: "run-1"}},
		{"empty action", audit.Event{Actor: "cli", Action: "", Resource: "run-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := audit.ValidateEvent(tc.event); !errors.Is(err, audit.ErrEmptyField) {
				t.Fatalf("ValidateEvent(%+v) err = %v, want ErrEmptyField", tc.event, err)
			}
		})
	}
}

func TestValidateEventRejectsSecrets(t *testing.T) {
	tests := []struct {
		name  string
		event audit.Event
	}{
		{
			name: "sensitive detail key",
			event: audit.Event{
				Actor:    "cli",
				Action:   audit.ActionCapOverrideEnabled,
				Resource: "run-1",
				Detail:   map[string]string{"authorization": "Bearer abc"},
			},
		},
		{
			name: "password key any case",
			event: audit.Event{
				Actor:    "cli",
				Action:   audit.ActionCapOverrideEnabled,
				Resource: "run-1",
				Detail:   map[string]string{"Password": "hunter2"},
			},
		},
		{
			name: "secret token in resource",
			event: audit.Event{
				Actor:    "cli",
				Action:   audit.ActionScenarioStarted,
				Resource: "token=abcdef",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := audit.ValidateEvent(tc.event); !errors.Is(err, audit.ErrSecretInEvent) {
				t.Fatalf("ValidateEvent(%+v) err = %v, want ErrSecretInEvent", tc.event, err)
			}
		})
	}
}

func TestValidateEventAcceptsCleanEvent(t *testing.T) {
	e := audit.Event{
		Actor:    "cli",
		Action:   audit.ActionScenarioStarted,
		Resource: "run-1",
		Detail:   map[string]string{"agent_count": "3", "target_host": "internal.example"},
	}
	if err := audit.ValidateEvent(e); err != nil {
		t.Fatalf("ValidateEvent(clean) = %v, want nil", err)
	}
}

func mkRecord() audit.Record {
	return audit.Record{
		Seq:      7,
		Time:     time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
		AgentID:  "agent-A",
		Actor:    "cli",
		Action:   audit.ActionCapOverrideEnabled,
		Resource: "run-42",
		Detail:   map[string]string{"new_global_rps": "100", "old_global_rps": "50"},
		PrevHash: "00ff",
	}
}

func TestRecordHashIsDeterministic(t *testing.T) {
	r := mkRecord()
	h1 := audit.HashRecordForTest(r)
	h2 := audit.HashRecordForTest(r)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("hash hex length = %d, want 64 (sha256)", len(h1))
	}
}

func TestRecordHashChangesWithEveryField(t *testing.T) {
	base := audit.HashRecordForTest(mkRecord())

	mutators := map[string]func(*audit.Record){
		"seq":      func(r *audit.Record) { r.Seq = 8 },
		"time":     func(r *audit.Record) { r.Time = r.Time.Add(time.Second) },
		"agentID":  func(r *audit.Record) { r.AgentID = "agent-B" },
		"actor":    func(r *audit.Record) { r.Actor = "ui" },
		"action":   func(r *audit.Record) { r.Action = audit.ActionScenarioStopped },
		"resource": func(r *audit.Record) { r.Resource = "run-43" },
		"detailV":  func(r *audit.Record) { r.Detail["new_global_rps"] = "101" },
		"detailK":  func(r *audit.Record) { delete(r.Detail, "old_global_rps"); r.Detail["older_global_rps"] = "50" },
		"prevHash": func(r *audit.Record) { r.PrevHash = "00fe" },
	}
	for name, mut := range mutators {
		t.Run(name, func(t *testing.T) {
			r := mkRecord()
			mut(&r)
			if audit.HashRecordForTest(r) == base {
				t.Fatalf("mutating %s did not change the hash", name)
			}
		})
	}
}

func TestRecordHashStableAcrossDetailMapOrder(t *testing.T) {
	r1 := mkRecord()
	r1.Detail = map[string]string{"a": "1", "b": "2", "c": "3"}
	r2 := mkRecord()
	r2.Detail = map[string]string{"c": "3", "a": "1", "b": "2"}
	if audit.HashRecordForTest(r1) != audit.HashRecordForTest(r2) {
		t.Fatal("hash must be independent of Detail map insertion order")
	}
}

func readRecords(t *testing.T, path string) []audit.Record {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 — path comes from t.TempDir(), not user input
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var recs []audit.Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r audit.Record
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatalf("unmarshal record: %v", err)
		}
		recs = append(recs, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return recs
}

// mustAppend appends an event and fails the test on error, collapsing the
// repetitive inline error check at call sites.
func mustAppend(t *testing.T, sink audit.Sink, e audit.Event) {
	t.Helper()
	if err := sink.Append(e); err != nil {
		t.Fatalf("append %s/%s: %v", e.Action, e.Resource, err)
	}
}

// assertRecordFields checks per-record Seq (0-based monotonic), AgentID, and the
// injected-clock timestamps.
func assertRecordFields(t *testing.T, recs []audit.Record, agentID string, wantTimes []time.Time) {
	t.Helper()
	if len(recs) != len(wantTimes) {
		t.Fatalf("got %d records, want %d", len(recs), len(wantTimes))
	}
	for i, r := range recs {
		if r.Seq != uint64(i) {
			t.Errorf("record %d Seq = %d, want %d", i, r.Seq, i)
		}
		if r.AgentID != agentID {
			t.Errorf("record %d AgentID = %q, want %s", i, r.AgentID, agentID)
		}
		if !r.Time.Equal(wantTimes[i]) {
			t.Errorf("record %d Time = %v, want %v (injected clock)", i, r.Time, wantTimes[i])
		}
	}
}

// assertHashChain checks that record 0 carries the fixed genesis PrevHash and
// that every subsequent PrevHash equals the prior record's Hash.
func assertHashChain(t *testing.T, recs []audit.Record) {
	t.Helper()
	if len(recs) == 0 {
		t.Fatal("no records to verify the hash chain")
	}
	if recs[0].PrevHash == "" {
		t.Error("record 0 PrevHash should be the fixed genesis hash, not empty")
	}
	for i := 1; i < len(recs); i++ {
		if recs[i].PrevHash != recs[i-1].Hash {
			t.Errorf("record %d PrevHash %q != record %d Hash %q", i, recs[i].PrevHash, i-1, recs[i-1].Hash)
		}
	}
}

func TestFileSinkAppendAndVerify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))

	sink, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	mustAppend(t, sink, audit.Event{Actor: "cli", Action: audit.ActionScenarioStarted, Resource: "run-1"})
	clk.Advance(2 * time.Minute)
	mustAppend(t, sink, audit.Event{
		Actor:    "ui",
		Action:   audit.ActionCapOverrideEnabled,
		Resource: "run-1",
		Detail:   map[string]string{"new_global_rps": "100"},
	})
	clk.Advance(time.Minute)
	mustAppend(t, sink, audit.Event{Actor: "cli", Action: audit.ActionScenarioStopped, Resource: "run-1"})

	if err := sink.Verify(); err != nil {
		t.Fatalf("Verify after 3 appends: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readRecords(t, path)
	wantTimes := []time.Time{
		time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 11, 9, 2, 0, 0, time.UTC),
		time.Date(2026, 6, 11, 9, 3, 0, 0, time.UTC),
	}
	assertRecordFields(t, recs, "agent-A", wantTimes)
	assertHashChain(t, recs)
}

func TestFileSinkAppendRejectsSecretAndDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	sink, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	defer sink.Close()

	err = sink.Append(audit.Event{
		Actor:    "cli",
		Action:   audit.ActionScenarioStarted,
		Resource: "run-1",
		Detail:   map[string]string{"password": "hunter2"},
	})
	if !errors.Is(err, audit.ErrSecretInEvent) {
		t.Fatalf("Append with secret err = %v, want ErrSecretInEvent", err)
	}
	if recs := readRecords(t, path); len(recs) != 0 {
		t.Fatalf("rejected secret event must NOT be written; found %d records", len(recs))
	}
}

func TestFileSinkDetectsMutatedMiddleRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	sink, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	for i := 0; i < 5; i++ {
		clk.Advance(time.Minute)
		if err := sink.Append(audit.Event{
			Actor:    "cli",
			Action:   audit.ActionScenarioPatched,
			Resource: "run-1",
			Detail:   map[string]string{"step": fmt.Sprintf("%d", i)},
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if err := sink.Verify(); err != nil {
		t.Fatalf("Verify clean chain: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Tamper: rewrite the middle record's Resource WITHOUT recomputing its hash.
	recs := readRecords(t, path)
	if len(recs) != 5 {
		t.Fatalf("expected 5 records, got %d", len(recs))
	}
	const tamperedSeq = 2
	recs[tamperedSeq].Resource = "run-EVIL"
	var buf []byte
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatalf("rewrite tampered file: %v", err)
	}

	// A fresh sink must refuse to open a tampered chain.
	_, err = audit.NewFileSink(path, clk, "agent-A")
	if !errors.Is(err, audit.ErrChainBroken) {
		t.Fatalf("NewFileSink on tampered chain err = %v, want ErrChainBroken", err)
	}
}

// appendN appends n patched-scenario records to a live sink, advancing the clock
// each time. It is a helper for the tail-tampering tests below.
func appendN(t *testing.T, sink audit.Sink, clk *fakeClock, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		clk.Advance(time.Minute)
		if err := sink.Append(audit.Event{
			Actor:    "cli",
			Action:   audit.ActionScenarioPatched,
			Resource: "run-1",
			Detail:   map[string]string{"step": fmt.Sprintf("%d", i)},
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
}

// C1: deleting the last on-disk record under a LIVE sink must be caught by Verify,
// because the on-disk tail no longer matches the in-memory tail. replay() alone
// would not catch this — the truncated chain is internally self-consistent.
func TestFileSinkVerifyDetectsLiveTailTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	sink, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	defer sink.Close()

	const n = 5
	appendN(t, sink, clk, n)
	if err := sink.Verify(); err != nil {
		t.Fatalf("Verify clean chain: %v", err)
	}

	// Delete the last line on disk, bypassing the sink.
	recs := readRecords(t, path)
	if len(recs) != n {
		t.Fatalf("expected %d records, got %d", n, len(recs))
	}
	var buf []byte
	for _, r := range recs[:n-1] { // drop the tail record
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatalf("rewrite truncated file: %v", err)
	}

	// The live sink still believes the chain has n records; Verify must catch it.
	if err := sink.Verify(); !errors.Is(err, audit.ErrChainBroken) {
		t.Fatalf("Verify after live tail truncation err = %v, want ErrChainBroken", err)
	}
}

// C1: appending a forged record that correctly chains off the real tail (written
// directly to the file, bypassing the sink) must be caught by Verify, because the
// on-disk tail advances past the in-memory tail. The forged record is internally
// valid, so replay() alone would accept it.
func TestFileSinkVerifyDetectsForgedTailAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	sink, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	defer sink.Close()

	const n = 3
	appendN(t, sink, clk, n)
	if err := sink.Verify(); err != nil {
		t.Fatalf("Verify clean chain: %v", err)
	}

	// Forge a record that chains off the genuine tail and recompute its hash, so it
	// passes replay()'s internal checks. Append it out-of-band.
	recs := readRecords(t, path)
	tail := recs[len(recs)-1]
	forged := audit.Record{
		Seq:      tail.Seq + 1,
		Time:     time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC),
		AgentID:  "agent-A",
		Actor:    "cli",
		Action:   audit.ActionScenarioStopped,
		Resource: "run-1",
		PrevHash: tail.Hash,
	}
	forged.Hash = audit.HashRecordForTest(forged)
	var buf []byte
	for _, r := range append(recs, forged) {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatalf("rewrite file with forged tail: %v", err)
	}

	// The live sink's in-memory tail is still at seq n; Verify must catch the
	// out-of-band extension even though the forged record is internally valid.
	if err := sink.Verify(); !errors.Is(err, audit.ErrChainBroken) {
		t.Fatalf("Verify after forged tail append err = %v, want ErrChainBroken", err)
	}
}

func TestFileSinkReopenContinuesChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))

	sink1, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink 1: %v", err)
	}
	if err := sink1.Append(audit.Event{Actor: "cli", Action: audit.ActionScenarioStarted, Resource: "run-1"}); err != nil {
		t.Fatalf("append a: %v", err)
	}
	clk.Advance(time.Minute)
	if err := sink1.Append(audit.Event{Actor: "cli", Action: audit.ActionCapOverrideEnabled, Resource: "run-1"}); err != nil {
		t.Fatalf("append b: %v", err)
	}
	if err := sink1.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}

	// Reopen: recovered seq must be 2, and the new record must link to the old tail.
	sink2, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink 2 (reopen): %v", err)
	}
	clk.Advance(time.Minute)
	if err := sink2.Append(audit.Event{Actor: "cli", Action: audit.ActionScenarioStopped, Resource: "run-1"}); err != nil {
		t.Fatalf("append c after reopen: %v", err)
	}
	if err := sink2.Verify(); err != nil {
		t.Fatalf("Verify after reopen: %v", err)
	}
	if err := sink2.Close(); err != nil {
		t.Fatalf("Close 2: %v", err)
	}

	recs := readRecords(t, path)
	if len(recs) != 3 {
		t.Fatalf("got %d records after reopen, want 3", len(recs))
	}
	if recs[2].Seq != 2 {
		t.Errorf("reopened record Seq = %d, want 2", recs[2].Seq)
	}
	if recs[2].PrevHash != recs[1].Hash {
		t.Errorf("reopened record PrevHash %q != prior Hash %q", recs[2].PrevHash, recs[1].Hash)
	}
}

func TestFileSinkConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	clk := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	sink, err := audit.NewFileSink(path, clk, "agent-A")
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	defer sink.Close()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = sink.Append(audit.Event{
				Actor:    "cli",
				Action:   audit.ActionScenarioPatched,
				Resource: "run-1",
				Detail:   map[string]string{"worker": fmt.Sprintf("%d", i)},
			})
		}(i)
	}
	wg.Wait()

	if err := sink.Verify(); err != nil {
		t.Fatalf("Verify after concurrent appends: %v", err)
	}
	recs := readRecords(t, path)
	if len(recs) != n {
		t.Fatalf("got %d records, want %d", len(recs), n)
	}
	// Sequence numbers must be a contiguous 0..n-1 with no gaps/dupes.
	seen := make(map[uint64]bool, n)
	for _, r := range recs {
		if seen[r.Seq] {
			t.Fatalf("duplicate Seq %d", r.Seq)
		}
		seen[r.Seq] = true
	}
	for i := uint64(0); i < n; i++ {
		if !seen[i] {
			t.Fatalf("missing Seq %d", i)
		}
	}
}
