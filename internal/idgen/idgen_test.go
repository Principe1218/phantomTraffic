package idgen

import (
	"strings"
	"testing"
)

// urlSafeAlphabet is base64 RawURLEncoding: A-Z a-z 0-9 - _  (no '+', '/', or '=').
const urlSafeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func isURLSafe(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(urlSafeAlphabet, r) {
			return false
		}
	}
	return true
}

func TestGenerators_ShapeAndCharset(t *testing.T) {
	tests := []struct {
		name    string
		gen     func() (string, error)
		wantLen int // RawURLEncoding length of 16 bytes = ceil(16*8/6) = 22
	}{
		{"SessionID", SessionID, 22},
		{"CorrelationID", CorrelationID, 22},
		{"Nonce16", func() (string, error) { return Nonce(16) }, 22},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.gen()
			if err != nil {
				t.Fatalf("%s returned error: %v", tc.name, err)
			}
			if len(got) != tc.wantLen {
				t.Errorf("%s len = %d, want %d (id=%q)", tc.name, len(got), tc.wantLen, got)
			}
			if !isURLSafe(got) {
				t.Errorf("%s = %q is not URL-safe", tc.name, got)
			}
		})
	}
}

func TestGenerators_Unique(t *testing.T) {
	const draws = 10000
	for _, tc := range []struct {
		name string
		gen  func() (string, error)
	}{
		{"SessionID", SessionID},
		{"CorrelationID", CorrelationID},
		{"Nonce16", func() (string, error) { return Nonce(16) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(map[string]struct{}, draws)
			for i := 0; i < draws; i++ {
				id, err := tc.gen()
				if err != nil {
					t.Fatalf("draw %d error: %v", i, err)
				}
				if _, dup := seen[id]; dup {
					t.Fatalf("collision at draw %d: %q", i, id)
				}
				seen[id] = struct{}{}
			}
		})
	}
}

func TestNonce_RejectsNonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -16} {
		if _, err := Nonce(n); err == nil {
			t.Errorf("Nonce(%d) = nil error, want error", n)
		}
	}
}

func TestGenerators_RandReadFailure(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	randRead = func(b []byte) (int, error) {
		return 0, errReadStub
	}
	for _, tc := range []struct {
		name string
		gen  func() (string, error)
	}{
		{"SessionID", SessionID},
		{"CorrelationID", CorrelationID},
		{"Nonce16", func() (string, error) { return Nonce(16) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.gen()
			if err == nil {
				t.Fatalf("%s returned nil error on rand failure (id=%q)", tc.name, got)
			}
			if got != "" {
				t.Errorf("%s returned non-empty id %q on rand failure", tc.name, got)
			}
		})
	}
}

var errReadStub = stubErr("stub crypto/rand failure")

type stubErr string

func (e stubErr) Error() string { return string(e) }
