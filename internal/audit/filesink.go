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
	// Verify re-reads the entire log and confirms every record's hash and the
	// PrevHash linkage. Returns ErrChainBroken on the first inconsistency.
	Verify() error
	// Close releases the underlying file. Idempotent.
	Close() error
}

// FileSink is a file-backed, append-only, hash-chained Sink (design §5.5).
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
		if jsonErr := json.Unmarshal(line, &r); jsonErr != nil {
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

// Verify implements Sink by re-reading the whole file and walking the chain.
func (s *FileSink) Verify() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.f.Sync(); err != nil {
		return fmt.Errorf("audit: fsync before verify: %w", err)
	}
	_, _, err := replay(s.f.Name())
	return err
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
