package behavior

import (
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

func TestDefaultFingerprintPoolLoadsAndValidates(t *testing.T) {
	pool, err := DefaultFingerprintPool()
	if err != nil {
		t.Fatalf("DefaultFingerprintPool: %v", err)
	}
	if pool.Len() < 8 {
		t.Fatalf("expected >=8 embedded fingerprints, got %d", pool.Len())
	}
}

func TestFingerprintPickIsDeterministic(t *testing.T) {
	pool, err := DefaultFingerprintPool()
	if err != nil {
		t.Fatal(err)
	}
	r := rng.NewFake(rng.FakeScript{Ints: []int{0}})
	if pool.Pick(r).UserAgent == "" {
		t.Fatal("picked an empty user agent")
	}
}

func TestNewFingerprintPoolRejectsEmpty(t *testing.T) {
	if _, err := NewFingerprintPool(nil); err == nil {
		t.Fatal("expected error for an empty pool")
	}
}

func TestValidateFingerprint(t *testing.T) {
	chrome := Fingerprint{
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		SecCHUA:         `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`,
		SecCHUAPlatform: `"Windows"`,
		AcceptLanguage:  "en-US,en;q=0.9",
	}
	if err := validateFingerprint(chrome); err != nil {
		t.Fatalf("valid Chrome fingerprint rejected: %v", err)
	}
	// Firefox sends no Sec-CH-UA; that is allowed.
	firefox := Fingerprint{
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0",
		AcceptLanguage: "en-US,en;q=0.5",
	}
	if err := validateFingerprint(firefox); err != nil {
		t.Fatalf("valid Firefox fingerprint rejected: %v", err)
	}

	bad := []struct {
		name string
		f    Fingerprint
	}{
		{"empty user agent", Fingerprint{UserAgent: "", AcceptLanguage: "en"}},
		{"CRLF header split", Fingerprint{UserAgent: "Chrome/126.0\r\nX-Injected: 1", AcceptLanguage: "en"}},
		{"UA/Sec-CH-UA major mismatch", Fingerprint{
			UserAgent: "Chrome/126.0.0.0", SecCHUA: `"Google Chrome";v="125"`, SecCHUAPlatform: `"Windows"`, AcceptLanguage: "en"}},
		{"Sec-CH-UA without platform", Fingerprint{
			UserAgent: "Chrome/126.0.0.0", SecCHUA: `"Google Chrome";v="126"`, AcceptLanguage: "en"}},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateFingerprint(tt.f); err == nil {
				t.Fatalf("expected rejection for %q", tt.name)
			}
		})
	}
}
