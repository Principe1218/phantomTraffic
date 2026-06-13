package behavior

import (
	"math"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// fakeRand is a deterministic, scripted rng.Rand for tests. Each draw method pops
// the next value from its slice; running past the end panics so a miscounted test
// fails loudly instead of silently reusing a zero value.
type fakeRand struct {
	floats []float64 // consumed by Float64
	norms  []float64 // consumed by NormFloat64
	exps   []float64 // consumed by ExpFloat64
	ints   []int     // consumed by IntN
	i63    []int64   // consumed by Int63n
}

func (f *fakeRand) pop(s *[]float64, who string) float64 {
	if len(*s) == 0 {
		panic("fakeRand: " + who + " called more times than scripted")
	}
	v := (*s)[0]
	*s = (*s)[1:]
	return v
}

func (f *fakeRand) Float64() float64     { return f.pop(&f.floats, "Float64") }
func (f *fakeRand) NormFloat64() float64 { return f.pop(&f.norms, "NormFloat64") }
func (f *fakeRand) ExpFloat64() float64  { return f.pop(&f.exps, "ExpFloat64") }

func (f *fakeRand) IntN(n int) int {
	if len(f.ints) == 0 {
		panic("fakeRand: IntN called more times than scripted")
	}
	v := f.ints[0]
	f.ints = f.ints[1:]
	return v
}

func (f *fakeRand) Int63n(n int64) int64 {
	if len(f.i63) == 0 {
		panic("fakeRand: Int63n called more times than scripted")
	}
	v := f.i63[0]
	f.i63 = f.i63[1:]
	return v
}

func (f *fakeRand) Perm(n int) []int { return nil }
func (f *fakeRand) Split() rng.Rand  { return f }

// compile-time proof the double satisfies the real interface.
var _ rng.Rand = (*fakeRand)(nil)

func TestFakeRandReplaysScriptedSequence(t *testing.T) {
	f := &fakeRand{floats: []float64{0.25, 0.75}, norms: []float64{-1.0}, exps: []float64{2.0}}
	if got := f.Float64(); got != 0.25 {
		t.Fatalf("Float64 #1 = %v, want 0.25", got)
	}
	if got := f.Float64(); got != 0.75 {
		t.Fatalf("Float64 #2 = %v, want 0.75", got)
	}
	if got := f.NormFloat64(); got != -1.0 {
		t.Fatalf("NormFloat64 = %v, want -1.0", got)
	}
	if got := f.ExpFloat64(); got != 2.0 {
		t.Fatalf("ExpFloat64 = %v, want 2.0", got)
	}

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on Float64 overrun, got none")
		}
	}()
	f.Float64() // floats exhausted ⇒ panic
}

func TestConstantSample(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want time.Duration
	}{
		{"positive", 500 * time.Millisecond, 500 * time.Millisecond},
		{"zero", 0, 0},
		{"negative clamps to zero", -2 * time.Second, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Empty fakeRand: Constant must not draw — any draw panics and fails the test.
			c := Constant{D: tc.d}
			if got := c.Sample(&fakeRand{}); got != tc.want {
				t.Fatalf("Constant{%v}.Sample = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
	if got := (Constant{}).Name(); got != "constant" {
		t.Fatalf("Constant.Name = %q, want %q", got, "constant")
	}
}

func TestUniformSample(t *testing.T) {
	cases := []struct {
		name string
		min  time.Duration
		max  time.Duration
		draw float64
		want time.Duration
	}{
		{"low end", 1 * time.Second, 3 * time.Second, 0.0, 1 * time.Second},
		{"midpoint", 1 * time.Second, 3 * time.Second, 0.5, 2 * time.Second},
		{"near top", 1 * time.Second, 3 * time.Second, 0.9999, 1*time.Second + time.Duration(0.9999*float64(2*time.Second))},
		{"degenerate max<=min", 4 * time.Second, 4 * time.Second, 0.5, 4 * time.Second},
		{"negative min clamps", -5 * time.Second, -1 * time.Second, 0.0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRand{}
			// Only the non-degenerate cases consume a Float64 draw.
			if tc.max-tc.min > 0 {
				f.floats = []float64{tc.draw}
			}
			u := Uniform{Min: tc.min, Max: tc.max}
			if got := u.Sample(f); got != tc.want {
				t.Fatalf("Uniform{%v,%v}.Sample(draw=%v) = %v, want %v", tc.min, tc.max, tc.draw, got, tc.want)
			}
		})
	}
	if got := (Uniform{}).Name(); got != "uniform" {
		t.Fatalf("Uniform.Name = %q, want %q", got, "uniform")
	}
}

func TestNormalSample(t *testing.T) {
	cases := []struct {
		name string
		n    Normal
		z    float64 // scripted NormFloat64 draw
		want time.Duration
	}{
		{
			name: "z=0 returns mean",
			n:    Normal{Mean: 2 * time.Second, StdDev: 500 * time.Millisecond, Min: 0, Max: 10 * time.Second},
			z:    0.0,
			want: 2 * time.Second,
		},
		{
			name: "positive z shifts up by one stddev",
			n:    Normal{Mean: 2 * time.Second, StdDev: 500 * time.Millisecond, Min: 0, Max: 10 * time.Second},
			z:    1.0,
			want: 2*time.Second + 500*time.Millisecond,
		},
		{
			name: "negative tail clamps to Min",
			n:    Normal{Mean: 1 * time.Second, StdDev: 1 * time.Second, Min: 200 * time.Millisecond, Max: 10 * time.Second},
			z:    -5.0, // 1s - 5s = -4s, below Min
			want: 200 * time.Millisecond,
		},
		{
			name: "upper tail clamps to Max",
			n:    Normal{Mean: 5 * time.Second, StdDev: 2 * time.Second, Min: 0, Max: 6 * time.Second},
			z:    3.0, // 5s + 6s = 11s, above Max
			want: 6 * time.Second,
		},
		{
			name: "Min below zero still clamps non-negative",
			n:    Normal{Mean: 0, StdDev: 1 * time.Second, Min: -10 * time.Second, Max: 10 * time.Second},
			z:    -2.0, // 0 - 2s = -2s, above Min(-10s) but negative
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRand{norms: []float64{tc.z}}
			if got := tc.n.Sample(f); got != tc.want {
				t.Fatalf("Normal.Sample(z=%v) = %v, want %v", tc.z, got, tc.want)
			}
		})
	}
	if got := (Normal{}).Name(); got != "normal" {
		t.Fatalf("Normal.Name = %q, want %q", got, "normal")
	}
}

func TestLogNormalSample(t *testing.T) {
	cases := []struct {
		name  string
		l     LogNormal
		z     float64       // scripted NormFloat64 draw
		scale time.Duration // effective scale used to compute want
	}{
		{
			name:  "z=0 median at exp(Mu)*Scale",
			l:     LogNormal{Mu: 0, Sigma: 0.5, Scale: time.Second},
			z:     0.0,
			scale: time.Second,
		},
		{
			name:  "positive z skews right",
			l:     LogNormal{Mu: 0, Sigma: 0.5, Scale: time.Second},
			z:     2.0,
			scale: time.Second,
		},
		{
			name:  "negative z stays positive (exp never negative)",
			l:     LogNormal{Mu: -1, Sigma: 1.0, Scale: time.Second},
			z:     -3.0,
			scale: time.Second,
		},
		{
			name:  "zero Scale defaults to one second",
			l:     LogNormal{Mu: 0.2, Sigma: 0.3},
			z:     1.0,
			scale: time.Second,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRand{norms: []float64{tc.z}}
			want := time.Duration(math.Exp(tc.l.Mu+tc.l.Sigma*tc.z) * float64(tc.scale))
			if want < 0 {
				want = 0
			}
			if got := tc.l.Sample(f); got != want {
				t.Fatalf("LogNormal.Sample(z=%v) = %v, want %v", tc.z, got, want)
			}
		})
	}
	if got := (LogNormal{}).Name(); got != "lognormal" {
		t.Fatalf("LogNormal.Name = %q, want %q", got, "lognormal")
	}
}

func TestExponentialSample(t *testing.T) {
	cases := []struct {
		name string
		mean time.Duration
		draw float64 // scripted ExpFloat64 draw (unit-rate exponential)
		want time.Duration
	}{
		{"zero draw", 2 * time.Second, 0.0, 0},
		{"unit draw equals mean", 2 * time.Second, 1.0, 2 * time.Second},
		{"long tail", 1 * time.Second, 3.5, time.Duration(3.5 * float64(time.Second))},
		{"negative mean clamps to zero", -2 * time.Second, 1.0, 0},
		{"zero mean is always zero", 0, 4.2, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRand{exps: []float64{tc.draw}}
			e := Exponential{Mean: tc.mean}
			if got := e.Sample(f); got != tc.want {
				t.Fatalf("Exponential{%v}.Sample(draw=%v) = %v, want %v", tc.mean, tc.draw, got, tc.want)
			}
		})
	}
	if got := (Exponential{}).Name(); got != "exponential" {
		t.Fatalf("Exponential.Name = %q, want %q", got, "exponential")
	}
}

// compile-time proof every impl satisfies Distribution.
var (
	_ Distribution = Constant{}
	_ Distribution = Uniform{}
	_ Distribution = Normal{}
	_ Distribution = LogNormal{}
	_ Distribution = Exponential{}
)

func TestSampleNeverNegative(t *testing.T) {
	cases := []struct {
		name string
		dist Distribution
		fr   *fakeRand
	}{
		{"Constant negative", Constant{D: -3 * time.Second}, &fakeRand{}},
		{"Uniform negative window", Uniform{Min: -5 * time.Second, Max: -1 * time.Second}, &fakeRand{floats: []float64{0.5}}},
		{"Normal negative tail", Normal{Mean: 0, StdDev: 1 * time.Second, Min: -10 * time.Second, Max: 10 * time.Second}, &fakeRand{norms: []float64{-4.0}}},
		{"Exponential negative mean", Exponential{Mean: -2 * time.Second}, &fakeRand{exps: []float64{2.0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.dist.Sample(tc.fr); got < 0 {
				t.Fatalf("%s.Sample = %v, want >= 0", tc.dist.Name(), got)
			}
		})
	}
}

func TestNamesAreUnique(t *testing.T) {
	dists := []Distribution{Constant{}, Uniform{}, Normal{}, LogNormal{}, Exponential{}}
	seen := map[string]bool{}
	for _, d := range dists {
		name := d.Name()
		if name == "" {
			t.Fatalf("%T returned an empty Name()", d)
		}
		if seen[name] {
			t.Fatalf("duplicate Distribution name %q", name)
		}
		seen[name] = true
	}
}
