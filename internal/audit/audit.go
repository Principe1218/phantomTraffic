// Package audit provides PhantomTraffic's built-in append-only local audit sink
// for security-relevant events: lifecycle transitions, safety-cap overrides, and
// insecure-transport opt-ins (design §2, §5.5, §7; AGENTS.md §9 AU-2/AU-3).
//
// Each persisted Record carries the prior record's SHA-256 digest, forming a
// hash chain so post-hoc tampering with any record is detectable (design §5.5).
// Events carry actor / action / resource and a timestamp that the Sink stamps
// from the injected clock.Clock — callers never supply the time. Events must
// never contain secrets or PII: ValidateEvent rejects known-sensitive Detail keys
// and obvious secret markers before anything is written (AGENTS.md §3.1).
package audit

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Action is the closed vocabulary of audit-grade event names (design §5.5).
// It is a typed string so callers cannot pass an arbitrary action.
type Action string

const (
	// ActionTLSVerificationSkipped records a per-connection insecure-TLS opt-in (design §5.5, D4).
	ActionTLSVerificationSkipped Action = "tls.verification_skipped"
	// ActionSSHHostKeyUnverified records an SSH host-key verification bypass opt-in (design §5.5, D4).
	ActionSSHHostKeyUnverified Action = "ssh.host_key_unverified"
	// ActionCapOverrideEnabled records raising a compiled-in safety ceiling via the audited override flag (design §5.5, D2).
	ActionCapOverrideEnabled Action = "safety.cap_override_enabled"
	// ActionScenarioStarted records a run starting (design §5.5).
	ActionScenarioStarted Action = "scenario.started"
	// ActionScenarioStopped records a run stopping (design §5.5).
	ActionScenarioStopped Action = "scenario.stopped"
	// ActionScenarioPatched records a live ApplyPatch mutation (design §5.5, §5 ApplyPatch).
	ActionScenarioPatched Action = "scenario.patched"
)

// knownActions is the allowlist used by ValidateEvent (AGENTS.md §5.2 — allowlist, not denylist).
var knownActions = map[Action]struct{}{
	ActionTLSVerificationSkipped: {},
	ActionSSHHostKeyUnverified:   {},
	ActionCapOverrideEnabled:     {},
	ActionScenarioStarted:        {},
	ActionScenarioStopped:        {},
	ActionScenarioPatched:        {},
}

// Event is the caller-supplied input to Sink.Append. It deliberately carries NO
// timestamp and NO hash: the Sink stamps the authoritative time from the injected
// clock and computes the chain hash. Detail holds low-cardinality, non-sensitive
// context (e.g. agent_count, fields_changed); it must never contain secrets or PII.
type Event struct {
	Actor    string            // who initiated the action, e.g. "cli", "ui", "scheduler"
	Action   Action            // one of the known Action constants
	Resource string            // what was acted on, e.g. a run ID or target ID — never a credential
	Detail   map[string]string // optional scrubbed context; sensitive keys are rejected
}

// Record is the persisted, hash-chained form of an Event. One Record is written
// per line as a JSON object. Hash = sha256 over the canonical encoding of every
// field INCLUDING PrevHash, so altering any field (or reordering records) breaks
// the chain (design §5.5).
type Record struct {
	Seq      uint64            `json:"seq"`       // 0-based monotonic position in the chain
	Time     time.Time         `json:"time"`      // stamped by the injected clock.Clock
	AgentID  string            `json:"agent_id"`  // per-agent identity (design §7 / line 603)
	Actor    string            `json:"actor"`
	Action   Action            `json:"action"`
	Resource string            `json:"resource"`
	Detail   map[string]string `json:"detail,omitempty"`
	PrevHash string            `json:"prev_hash"` // hex digest of the previous record; genesis for Seq 0
	Hash     string            `json:"hash"`      // hex digest of this record (over all fields above)
}

// Sentinel errors. Messages are redaction-safe (never echo the offending secret).
var (
	// ErrSecretInEvent is returned when an Event field appears to contain a secret or PII.
	ErrSecretInEvent = errors.New("audit: event field appears to contain a secret or sensitive value")
	// ErrEmptyField is returned when a required Event field is empty or the action is unknown.
	ErrEmptyField = errors.New("audit: event has an empty required field or unknown action")
	// ErrChainBroken is returned by Verify when a record's stored hash does not match
	// its recomputed hash, or PrevHash does not match the prior record's Hash.
	ErrChainBroken = errors.New("audit: hash chain verification failed")
)

// sensitiveSubstrings are matched case-insensitively against Detail keys and against
// Actor/Resource/Detail-value content. Mirrors the scrubbing-handler denylist in
// design §5.5 (authorization, cookie, password, token, secret, key, credential).
var sensitiveSubstrings = []string{
	"authorization",
	"cookie",
	"password",
	"passwd",
	"secret",
	"token",
	"apikey",
	"api_key",
	"credential",
	"private_key",
	"bearer ",
}

func containsSensitive(s string) bool {
	low := strings.ToLower(s)
	for _, marker := range sensitiveSubstrings {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

// ValidateEvent enforces the no-secrets / no-empty-fields contract BEFORE any I/O
// (AGENTS.md §3.1, §5.2). It is called by every Sink implementation in Append, and
// is exported so callers can pre-check. Returned errors are redaction-safe.
func ValidateEvent(e Event) error {
	if strings.TrimSpace(e.Actor) == "" {
		return fmt.Errorf("%w: actor", ErrEmptyField)
	}
	if strings.TrimSpace(e.Resource) == "" {
		return fmt.Errorf("%w: resource", ErrEmptyField)
	}
	if _, ok := knownActions[e.Action]; !ok {
		return fmt.Errorf("%w: action %q", ErrEmptyField, e.Action)
	}
	if containsSensitive(e.Actor) {
		return fmt.Errorf("%w: in actor", ErrSecretInEvent)
	}
	if containsSensitive(e.Resource) {
		return fmt.Errorf("%w: in resource", ErrSecretInEvent)
	}
	for k, v := range e.Detail {
		if containsSensitive(k) {
			return fmt.Errorf("%w: detail key", ErrSecretInEvent)
		}
		if containsSensitive(v) {
			return fmt.Errorf("%w: detail value", ErrSecretInEvent)
		}
	}
	return nil
}
