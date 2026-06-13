package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// tokenBytes is 128 bits of entropy: enough that a collision across any feasible
// run is negligible, matching the design's "≥128-bit" session-token requirement.
const tokenBytes = 16

// randRead is the crypto/rand entropy seam. Production uses crypto/rand.Reader
// via rand.Read. Tests reassign it ONLY to exercise the read-failure path; it is
// never pointed at a non-cryptographic source.
var randRead = rand.Read

// token returns nBytes of crypto/rand entropy as a URL-safe, unpadded string.
func token(nBytes int) (string, error) {
	if nBytes <= 0 {
		return "", fmt.Errorf("idgen: token size must be positive, got %d", nBytes)
	}
	b := make([]byte, nBytes)
	n, err := randRead(b)
	if err != nil {
		return "", fmt.Errorf("idgen: read crypto/rand: %w", err)
	}
	if n != nBytes {
		return "", fmt.Errorf("idgen: short read from crypto/rand: got %d of %d bytes", n, nBytes)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SessionID returns a fresh, unguessable 128-bit session identifier.
// Callers convert with protocols.SessionID(id) (design §2: SessionID is a string).
func SessionID() (string, error) {
	return token(tokenBytes)
}

// CorrelationID returns a fresh 128-bit correlation/request id used to thread one
// request through structured logs (design §2; §7 request_id field).
func CorrelationID() (string, error) {
	return token(tokenBytes)
}

// ErrNonceSize is returned by Nonce when nBytes is not positive.
var ErrNonceSize = errors.New("idgen: nonce size must be positive")

// Nonce returns a fresh crypto/rand nonce of nBytes raw bytes, URL-safe encoded.
func Nonce(nBytes int) (string, error) {
	if nBytes <= 0 {
		return "", ErrNonceSize
	}
	return token(nBytes)
}
