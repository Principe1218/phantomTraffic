package scenario

import "testing"

func TestWeightBasisString(t *testing.T) {
    tests := []struct {
        name string
        in   WeightBasis
        want string
    }{
        {"vuser_population is the zero value", WeightByVuserPopulation, "vuser_population"},
        {"concurrency", WeightByConcurrency, "concurrency"},
        {"request_rate", WeightByRequestRate, "request_rate"},
        {"out-of-range falls back to unknown", WeightBasis(200), "unknown"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := tt.in.String(); got != tt.want {
                t.Fatalf("WeightBasis(%d).String() = %q, want %q", tt.in, got, tt.want)
            }
        })
    }
}

func TestWeightBasisIotaOrderIsStable(t *testing.T) {
    // The zero value IS the default for an empty weight_basis YAML field; lock it.
    if WeightByVuserPopulation != 0 || WeightByConcurrency != 1 || WeightByRequestRate != 2 {
        t.Fatalf("WeightBasis iota changed: vuser=%d concurrency=%d request=%d",
            WeightByVuserPopulation, WeightByConcurrency, WeightByRequestRate)
    }
}

func TestParseWeightBasis(t *testing.T) {
    tests := []struct {
        name   string
        in     string
        want   WeightBasis
        wantOK bool
    }{
        {"empty defaults to vuser_population", "", WeightByVuserPopulation, true},
        {"vuser_population", "vuser_population", WeightByVuserPopulation, true},
        {"concurrency", "concurrency", WeightByConcurrency, true},
        {"request_rate", "request_rate", WeightByRequestRate, true},
        {"unknown value rejected", "bytes", WeightByVuserPopulation, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, ok := parseWeightBasis(tt.in)
            if ok != tt.wantOK {
                t.Fatalf("parseWeightBasis(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
            }
            if got != tt.want {
                t.Fatalf("parseWeightBasis(%q) = %v, want %v", tt.in, got, tt.want)
            }
        })
    }
}
