package rng

// FakeScript is the deterministic value tape for a fakeRand. Each slice feeds the
// matching method in order. Tests of Distributions, think-time, jitter, and burst
// shaping use this to make normal/exponential draws fully reproducible without a
// real generator (design §2, lines 318–320).
type FakeScript struct {
	Floats []float64
	Norms  []float64
	Exps   []float64
	Ints   []int
	Int63s []int64
	Perms  [][]int
}

// fakeRand returns scripted values in order. Out-drawing a script panics, because a
// test consuming more values than it scripted is a test bug, not a runtime concern.
type fakeRand struct {
	script                  FakeScript
	fi, ni, ei, ii, i63, pi int
}

// NewFake returns a Rand that replays the given script. The script is deep-copied so
// the returned value (and any Split child) cannot be mutated through the caller's
// slices.
func NewFake(s FakeScript) Rand {
	return &fakeRand{script: copyScript(s)}
}

func copyScript(s FakeScript) FakeScript {
	out := FakeScript{
		Floats: append([]float64(nil), s.Floats...),
		Norms:  append([]float64(nil), s.Norms...),
		Exps:   append([]float64(nil), s.Exps...),
		Ints:   append([]int(nil), s.Ints...),
		Int63s: append([]int64(nil), s.Int63s...),
	}
	if s.Perms != nil {
		out.Perms = make([][]int, len(s.Perms))
		for i, p := range s.Perms {
			out.Perms[i] = append([]int(nil), p...)
		}
	}
	return out
}

func (f *fakeRand) Float64() float64 {
	if f.fi >= len(f.script.Floats) {
		panic("rng: fakeRand Float64 script exhausted")
	}
	v := f.script.Floats[f.fi]
	f.fi++
	return v
}

func (f *fakeRand) NormFloat64() float64 {
	if f.ni >= len(f.script.Norms) {
		panic("rng: fakeRand NormFloat64 script exhausted")
	}
	v := f.script.Norms[f.ni]
	f.ni++
	return v
}

func (f *fakeRand) ExpFloat64() float64 {
	if f.ei >= len(f.script.Exps) {
		panic("rng: fakeRand ExpFloat64 script exhausted")
	}
	v := f.script.Exps[f.ei]
	f.ei++
	return v
}

func (f *fakeRand) IntN(n int) int {
	if f.ii >= len(f.script.Ints) {
		panic("rng: fakeRand IntN script exhausted")
	}
	v := f.script.Ints[f.ii]
	f.ii++
	return v
}

func (f *fakeRand) Int63n(n int64) int64 {
	if f.i63 >= len(f.script.Int63s) {
		panic("rng: fakeRand Int63n script exhausted")
	}
	v := f.script.Int63s[f.i63]
	f.i63++
	return v
}

func (f *fakeRand) Perm(n int) []int {
	if f.pi >= len(f.script.Perms) {
		panic("rng: fakeRand Perm script exhausted")
	}
	v := f.script.Perms[f.pi]
	f.pi++
	return append([]int(nil), v...)
}

// Split returns an independent fakeRand whose script is a fresh deep copy starting
// from the beginning, mirroring prodRand.Split's "independent child stream" contract
// while staying fully scripted/deterministic for tests.
func (f *fakeRand) Split() Rand {
	return &fakeRand{script: copyScript(f.script)}
}
