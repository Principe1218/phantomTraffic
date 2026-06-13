package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// Sink is the append-only audit log. Implementations are safe for concurrent use.
type Sink interface {
	// Append validates the event (rejecting secrets/empty fields), stamps the
	// authoritative timestamp from the injected clock, links it into the hash
	// chain, and durably writes it. It returns ErrSecretInEvent / ErrEmptyField
	// without writing anything when validation fails.
	Append(e Event) error
	// Verify re-reads the entire log, confirms every record's hash and PrevHash
	// linkage, and cross-checks the on-disk tail (sequence + hash) against this
	// sink's in-memory chain state. It detects interior mutation, reordering, and
	// in-process tail truncation / forged tail-extension, returning ErrChainBroken
	// on the first inconsistency. It does NOT, by itself, detect cross-restart tail
	// truncation or forged tail-extension — after a reopen there is no prior
	// in-memory tail to compare against; that requires an external anchor (see the
	// package doc and AGENTS.md §9).
	Verify() error
	// Close releases the underlying file. Idempotent.
	Close() error
}

// FileSink is a file-backed, append-only, hash-chained Sink (design §5.5).
//
// Tamper-evidence scope: while a FileSink is live, Verify detects interior record
// mutation, reordering, and tail truncation / forged tail-extension by cross-checking
// the on-disk tail against the in-memory nextSeq/tailHash. It does NOT, on its own,
// detect cross-restart tail truncation or forged tail-extension — that needs an
// external anchor (see the package doc and AGENTS.md §9).
type FileSink struct {
	mu       sync.Mutex
	f        *os.File
	clk      clock.Clock
	agentID  string
	nextSeq  uint64
	tailHash string
	closed   bool
}

var _ Sink = (*FileSink)(nil)

// NewFileSink opens (creating if absent) the append-only audit file at path with
// 0600 permissions. If the file already contains records, it replays and verifies
// the chain, recovering the next sequence number and the tail hash so appends
// continue the same chain. A corrupt existing chain fails fast with ErrChainBroken.
//
// Known non-enforcement: the sink does NOT enforce single-agent ownership across
// reopen. An existing chain whose tail record carries a different AgentID than the
// agentID passed here is accepted and continued; subsequent records are stamped
// with the new agentID. This is deliberate — enforcing it would break legitimate
// reopen by a differently-identified process — but it means a mixed-AgentID chain
// is not, by itself, evidence of tampering.
func NewFileSink(path string, clk clock.Clock, agentID string) (*FileSink, error) {
	if clk == nil {
		return nil, fmt.Errorf("audit: NewFileSink requires a non-nil clock")
	}
	// Recover/verify any existing chain first (read-only pass).
	nextSeq, tailHash, err := replay(path)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 — path is a caller-supplied log file path, not user input
	if err != nil {
		return nil, fmt.Errorf("audit: open %s: %w", path, err)
	}
	return &FileSink{
		f:        f,
		clk:      clk,
		agentID:  agentID,
		nextSeq:  nextSeq,
		tailHash: tailHash,
	}, nil
}

// replay scans an existing file, verifies the chain, and returns the next seq and
// the tail hash. A missing file is treated as an empty chain (genesis next).
func replay(path string) (nextSeq uint64, tailHash string, err error) {
	f, openErr := os.Open(path) // #nosec G304 — path is the audit log file, not user-controlled input
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return 0, genesisHash, nil
		}
		return 0, "", fmt.Errorf("audit: open %s: %w", path, openErr)
	}
	defer f.Close()

	prevHash := genesisHash
	var seq uint64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if json.Unmarshal(line, &r) != nil {
			return 0, "", fmt.Errorf("%w: record %d is not valid JSON", ErrChainBroken, seq)
		}
		if r.Seq != seq {
			return 0, "", fmt.Errorf("%w: record at position %d has Seq %d", ErrChainBroken, seq, r.Seq)
		}
		if r.PrevHash != prevHash {
			return 0, "", fmt.Errorf("%w: record %d PrevHash does not match prior Hash", ErrChainBroken, r.Seq)
		}
		if recordHash(r) != r.Hash {
			return 0, "", fmt.Errorf("%w: record %d Hash does not match its contents", ErrChainBroken, r.Seq)
		}
		prevHash = r.Hash
		seq++
	}
	if scanErr := sc.Err(); scanErr != nil {
		return 0, "", fmt.Errorf("audit: scanning %s: %w", path, scanErr)
	}
	return seq, prevHash, nil
}

// Append implements Sink.
func (s *FileSink) Append(e Event) error {
	if err := ValidateEvent(e); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("audit: append on closed sink")
	}

	rec := Record{
		Seq:      s.nextSeq,
		Time:     s.clk.Now().UTC(),
		AgentID:  s.agentID,
		Actor:    e.Actor,
		Action:   e.Action,
		Resource: e.Resource,
		Detail:   e.Detail,
		PrevHash: s.tailHash,
	}
	rec.Hash = recordHash(rec)

	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("audit: marshal record: %w", err)
	}
	line = append(line, '\n')
	if _, err := s.f.Write(line); err != nil {
		return fmt.Errorf("audit: write record: %w", err)
	}
	if err := s.f.Sync(); err != nil {
		return fmt.Errorf("audit: fsync: %w", err)
	}

	s.tailHash = rec.Hash
	s.nextSeq++
	return nil
}

// Verify implements Sink by re-reading the whole file, walking the chain, and
// cross-checking the on-disk tail against this sink's in-memory chain state. The
// internal walk (replay) catches interior mutation and reordering; the cross-check
// against nextSeq/tailHash additionally catches in-process tail truncation (a
// deleted last record) and forged tail-extension (a record appended out-of-band
// that chains off the real tail). See the package doc for the cross-restart limit.
func (s *FileSink) Verify() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.f.Sync(); err != nil {
		return fmt.Errorf("audit: fsync before verify: %w", err)
	}
	onDiskSeq, onDiskTail, err := replay(s.f.Name())
	if err != nil {
		return err
	}
	// Cross-check: the on-disk chain must end exactly where this sink believes it
	// does. A divergence means the tail was truncated or extended out-of-band.
	if onDiskSeq != s.nextSeq || onDiskTail != s.tailHash {
		return fmt.Errorf("%w: on-disk tail at seq %d diverges from in-memory tail at seq %d",
			ErrChainBroken, onDiskSeq, s.nextSeq)
	}
	return nil
}

// Close implements Sink. It is idempotent.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.f.Close()
}
