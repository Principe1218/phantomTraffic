package protocols

// NOTE: the Outcome value OutcomeCancelled and its string form "cancelled"
// are transcribed verbatim from the foundation design/plan (§2). They are
// load-bearing contract names other modules match exactly, so the en-GB
// spelling is intentional and required here. en-GB-ok

import (
	"fmt"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// Result is the SCRUBBED result envelope (design §2). It is safe to log,
// serialize, and stream to the UI: it carries NO secrets, headers, bodies,
// or stacks. The behavior-only feedback channel is the separate Observation
// (in-process only). Exactly one of the typed per-protocol metas is non-nil,
// matching Protocol — the allowlist for metadata is the TYPE, not a runtime
// key filter (AGENTS.md §5.2).
type Result struct {
	Protocol  ProtocolID    `json:"protocol"`
	Action    ActionKind    `json:"action"`
	Target    string        `json:"target"` // target ID, never credentials
	Session   SessionID     `json:"session"`
	Seq       uint64        `json:"seq"` // per-session monotonic; orders dashboard rows
	Nav       NavID         `json:"nav,omitempty"`
	Outcome   Outcome       `json:"outcome"`
	ErrClass  pterr.Class   `json:"errClass,omitempty"`
	ErrCode   string        `json:"errCode,omitempty"` // stable low-cardinality, e.g. "http.5xx"
	StartedAt time.Time     `json:"startedAt"`         // injected Clock
	Latency   time.Duration `json:"latency"`           // injected Clock
	BytesIn   int64         `json:"bytesIn"`
	BytesOut  int64         `json:"bytesOut"`

	// Exactly one is non-nil, matching Protocol.
	HTTP   *HTTPMeta   `json:"http,omitempty"`
	DNS    *DNSMeta    `json:"dns,omitempty"`
	SSH    *SSHMeta    `json:"ssh,omitempty"`
	Stream *StreamMeta `json:"stream,omitempty"`
}

// HTTPMeta is the typed HTTP slice of scrubbed result metadata (design §2).
type HTTPMeta struct {
	Status    int    `json:"status"`
	Method    string `json:"method"`
	Redirects int    `json:"redirects"`
}

// DNSMeta is the typed DNS slice of scrubbed result metadata (design §2).
type DNSMeta struct {
	Rcode    string `json:"rcode"`
	QType    string `json:"qtype"`
	CacheHit bool   `json:"cacheHit"`
	Answers  int    `json:"answers"`
}

// SSHMeta is the typed SSH slice of scrubbed result metadata (design §2).
// It carries an exit code and the verb name only — never stdout or keys.
type SSHMeta struct {
	ExitCode int    `json:"exitCode"`
	Action   string `json:"action"`
}

// StreamMeta is the typed streaming slice of scrubbed result metadata (design §2).
type StreamMeta struct {
	VariantBitrateKbps int `json:"variantBitrateKbps"`
	SegmentIndex       int `json:"segmentIndex"`
	BufferMs           int `json:"bufferMs"`
}

// Outcome is the coarse, non-revealing result bucket (design §2).
type Outcome uint8

const (
	OutcomeSuccess Outcome = iota
	OutcomeFailure
	OutcomeSkipped
	OutcomeCancelled
	OutcomePanicked
	OutcomeReconnect // pause/resume artifact; NOT a failure (design §5)
)

func (o Outcome) String() string {
	switch o {
	case OutcomeSuccess:
		return "success"
	case OutcomeFailure:
		return "failure"
	case OutcomeSkipped:
		return "skipped"
	case OutcomeCancelled:
		return "cancelled"
	case OutcomePanicked:
		return "panicked"
	case OutcomeReconnect:
		return "reconnect"
	default:
		return fmt.Sprintf("outcome(%d)", uint8(o))
	}
}
