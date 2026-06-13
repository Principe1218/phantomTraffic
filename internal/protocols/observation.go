package protocols

import "time"

// Observation is the IN-PROCESS-ONLY control-feedback channel for the
// behavior layer's branching/ABR/cache loops (design §2). It NEVER reaches
// StatsSink, logs, or the UI.
//
// LIFETIME/ALIASING CONTRACT: Observation is passed BY VALUE and contains
// ONLY value types and bounded scalars. It carries NO pointers into
// handler-owned memory. Sensitive fields are reduced to FIXED-SIZE DIGESTS
// at the handler boundary, never raw bytes (AGENTS.md §3.1):
//   - SSH stdout: a fixed BLAKE2b/SHA-256 digest + length + a bounded
//     (<=64-byte) charset-safe prefix for branching — never full output,
//     never secrets.
//   - HTTP: same-origin link PATHS only (host stripped), allowlist-filtered.
//
// Because it is a value with no aliasing, "handlers must not retain it" is
// satisfied STRUCTURALLY — there is nothing shared to retain.
type Observation struct {
	Throughput   float64       // bytes/sec of this action, via injected Clock (ABR signal)
	BufferHealth time.Duration // streaming: media buffered
	DNS          DNSObs        // value: TTLs, rcode, CNAME target, TC bit, record types
	HTTP         HTTPObs       // value: redirect counts, allowlisted same-origin link paths, form field names, set-cookie presence
	SSH          SSHObs        // value: exit code, bounded stdout digest+prefix
	Stream       StreamObs     // value: available variants, current variant
	Has          ObsMask       // which sub-structs are populated
}

// DNSObs is the by-value DNS feedback slice (design §2).
type DNSObs struct {
	TTLSeconds  uint32
	Rcode       string
	CNAMETarget string
	Truncated   bool // TC bit
	RecordTypes []string
}

// HTTPObs is the by-value HTTP feedback slice (design §2). SameOriginLinkPaths
// are already host-stripped and allowlist-filtered at the handler boundary.
type HTTPObs struct {
	RedirectCount       int
	SameOriginLinkPaths []string
	FormFieldNames      []string
	HasSetCookie        bool
}

// SSHObs is the by-value SSH feedback slice (design §2). It NEVER carries raw
// stdout: only a fixed digest, the length, and a bounded charset-safe prefix.
type SSHObs struct {
	ExitCode     int
	StdoutDigest [32]byte // fixed-size; BLAKE2b-256 or SHA-256 of stdout
	StdoutLen    int64
	StdoutPrefix string // bounded (<=64 bytes), charset-safe, for branching only
}

// StreamObs is the by-value streaming feedback slice (design §2).
type StreamObs struct {
	AvailableBitratesKbps []int
	CurrentBitrateKbps    int
}

// ObsMask is a bitmask of which Observation sub-structs are populated (design §2).
type ObsMask uint8

const (
	ObsDNS    ObsMask = 1 << iota // DNS sub-struct populated
	ObsHTTP                       // HTTP sub-struct populated
	ObsSSH                        // SSH sub-struct populated
	ObsStream                     // Stream sub-struct populated
)

// Set returns the mask with bit b set (value receiver — no mutation aliasing).
func (m ObsMask) Set(b ObsMask) ObsMask { return m | b }

// Has reports whether bit b is set.
func (m ObsMask) Has(b ObsMask) bool { return m&b != 0 }
