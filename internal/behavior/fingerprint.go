package behavior

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// Fingerprint is one internally-consistent set of HTTP identity headers. A
// session picks ONE at construction and threads it through every HTTP action —
// a real browser does not change UA mid-session (design §4).
type Fingerprint struct {
	UserAgent       string `json:"user_agent"`
	SecCHUA         string `json:"sec_ch_ua"`
	SecCHUAPlatform string `json:"sec_ch_ua_platform"`
	AcceptLanguage  string `json:"accept_language"`
}

// FingerprintPool is a validated, non-empty set of fingerprints.
type FingerprintPool interface {
	Pick(r rng.Rand) Fingerprint
	Len() int
}

type staticPool struct {
	prints []Fingerprint
}

func (p *staticPool) Len() int { return len(p.prints) }

// Pick draws one fingerprint uniformly. The pool is guaranteed non-empty by
// NewFingerprintPool, so IntN's argument is always positive.
func (p *staticPool) Pick(r rng.Rand) Fingerprint { return p.prints[r.IntN(len(p.prints))] }

//go:embed fingerprints.json
var fingerprintData []byte

// DefaultFingerprintPool decodes and validates the embedded fingerprint table.
func DefaultFingerprintPool() (FingerprintPool, error) {
	var prints []Fingerprint
	if err := json.Unmarshal(fingerprintData, &prints); err != nil {
		return nil, fmt.Errorf("behavior: decode embedded fingerprints: %w", err)
	}
	return NewFingerprintPool(prints)
}

// NewFingerprintPool validates every fingerprint and returns a defensively-copied
// pool. A pool MUST be non-empty.
func NewFingerprintPool(prints []Fingerprint) (FingerprintPool, error) {
	if len(prints) == 0 {
		return nil, fmt.Errorf("behavior: fingerprint pool must be non-empty")
	}
	for i, f := range prints {
		if err := validateFingerprint(f); err != nil {
			return nil, fmt.Errorf("behavior: fingerprint[%d]: %w", i, err)
		}
	}
	return &staticPool{prints: append([]Fingerprint(nil), prints...)}, nil
}

const fingerprintFieldMaxLen = 512

// validateFingerprint is the security-load-bearing check (AGENTS.md §5.2/§5.3):
// (1) injection safety — every header value is length-bounded and free of
// CR/LF/control bytes, so a fingerprint can never header-split; (2) internal
// consistency — when Sec-CH-UA is present (Chromium), the UA Chrome major must
// appear in it and the platform must be set (a real browser never mismatches).
func validateFingerprint(f Fingerprint) error {
	if err := checkHeaderValue("user_agent", f.UserAgent, true); err != nil {
		return err
	}
	if err := checkHeaderValue("accept_language", f.AcceptLanguage, true); err != nil {
		return err
	}
	if err := checkHeaderValue("sec_ch_ua", f.SecCHUA, false); err != nil {
		return err
	}
	if err := checkHeaderValue("sec_ch_ua_platform", f.SecCHUAPlatform, false); err != nil {
		return err
	}
	if f.SecCHUA == "" {
		return nil // non-Chromium (Firefox/Safari): no consistency constraint
	}
	if f.SecCHUAPlatform == "" {
		return fmt.Errorf("sec_ch_ua_platform must be set when sec_ch_ua is present")
	}
	major, ok := chromeMajorFromUA(f.UserAgent)
	if !ok {
		return fmt.Errorf("sec_ch_ua present but user_agent has no Chrome/<major> token")
	}
	if !secCHUAHasVersion(f.SecCHUA, major) {
		return fmt.Errorf("UA Chrome major %d not present in sec_ch_ua %q", major, f.SecCHUA)
	}
	return nil
}

// checkHeaderValue enforces injection safety for one header value. required=false
// permits the empty string (an absent optional header).
func checkHeaderValue(name, val string, required bool) error {
	if val == "" {
		if required {
			return fmt.Errorf("%s must be non-empty", name)
		}
		return nil
	}
	if len(val) > fingerprintFieldMaxLen {
		return fmt.Errorf("%s exceeds %d bytes", name, fingerprintFieldMaxLen)
	}
	if i := strings.IndexFunc(val, isUnsafeHeaderByte); i >= 0 {
		return fmt.Errorf("%s contains an unsafe byte at offset %d (CR/LF/control)", name, i)
	}
	return nil
}

// isUnsafeHeaderByte reports CR, LF, other C0 control bytes, and DEL. Printable
// ASCII and UTF-8 multibyte runes are allowed.
func isUnsafeHeaderByte(r rune) bool { return r == '\r' || r == '\n' || r < 0x20 || r == 0x7f }

// chromeMajorFromUA extracts the integer after "Chrome/", e.g. 126 from
// "...Chrome/126.0.0.0...". ok is false when there is no Chrome token.
func chromeMajorFromUA(ua string) (int, bool) {
	const marker = "Chrome/"
	i := strings.Index(ua, marker)
	if i < 0 {
		return 0, false
	}
	rest := ua[i+len(marker):]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:j])
	if err != nil {
		return 0, false
	}
	return n, true
}

// secCHUAHasVersion reports whether the Sec-CH-UA header contains a v="major"
// brand token (e.g. v="126").
func secCHUAHasVersion(secCHUA string, major int) bool {
	return strings.Contains(secCHUA, `v="`+strconv.Itoa(major)+`"`)
}
