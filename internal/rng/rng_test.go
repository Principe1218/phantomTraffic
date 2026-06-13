package rng

import (
	"math/rand/v2" // nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"reflect"
	"testing"
)

func TestNew_SameSeed_SameFloat64Sequence(t *testing.T) {
	a := New(42, 99)
	b := New(42, 99)
	for i := 0; i < 16; i++ {
		x, y := a.Float64(), b.Float64()
		if x != y {
			t.Fatalf("draw %d: same seed produced divergent Float64: %v != %v", i, x, y)
		}
		if x < 0 || x >= 1 {
			t.Fatalf("draw %d: Float64 out of [0,1): %v", i, x)
		}
	}
}

func TestNew_DifferentSeed_DifferentSequence(t *testing.T) {
	a := New(1, 1)
	b := New(2, 2)
	gotA := make([]float64, 8)
	gotB := make([]float64, 8)
	for i := range gotA {
		gotA[i] = a.Float64()
		gotB[i] = b.Float64()
	}
	if reflect.DeepEqual(gotA, gotB) {
		t.Fatalf("different seeds produced identical sequences: %v", gotA)
	}
}

func TestProdRand_DelegatesToStdlibZiggurat(t *testing.T) {
	const seed1, seed2 uint64 = 0xDEADBEEF, 0x1234567890ABCDEF

	// our Rand
	got := New(seed1, seed2)

	// an independently-constructed stdlib generator with the IDENTICAL seed
	want := rand.New(rand.NewPCG(seed1, seed2))

	// The draw ORDER below must exactly mirror the order we pull from `want`,
	// since both share one underlying PCG stream. Any reimplementation of the
	// Ziggurat (e.g. a hand-rolled Box-Muller) would diverge here.
	for i := 0; i < 32; i++ {
		switch i % 5 {
		case 0:
			if g, w := got.Float64(), want.Float64(); g != w {
				t.Fatalf("Float64 draw %d not delegated: %v != %v", i, g, w)
			}
		case 1:
			if g, w := got.NormFloat64(), want.NormFloat64(); g != w {
				t.Fatalf("NormFloat64 draw %d not delegated to stdlib Ziggurat: %v != %v", i, g, w)
			}
		case 2:
			if g, w := got.ExpFloat64(), want.ExpFloat64(); g != w {
				t.Fatalf("ExpFloat64 draw %d not delegated to stdlib Ziggurat: %v != %v", i, g, w)
			}
		case 3:
			if g, w := got.IntN(1000), want.IntN(1000); g != w {
				t.Fatalf("IntN draw %d not delegated: %v != %v", i, g, w)
			}
		case 4:
			if g, w := got.Int63n(1<<40), want.Int64N(1<<40); g != w {
				t.Fatalf("Int63n draw %d not delegated to Int64N: %v != %v", i, g, w)
			}
		}
	}
}

func TestProdRand_PermBoundsAndPermutation(t *testing.T) {
	r := New(5, 5)
	const n = 12
	p := r.Perm(n)
	if len(p) != n {
		t.Fatalf("Perm(%d) length = %d, want %d", n, len(p), n)
	}
	seen := make([]bool, n)
	for _, v := range p {
		if v < 0 || v >= n {
			t.Fatalf("Perm value %d out of range [0,%d)", v, n)
		}
		if seen[v] {
			t.Fatalf("Perm value %d duplicated", v)
		}
		seen[v] = true
	}
}

func TestSplit_ReproducibleChildStreams(t *testing.T) {
	// Two parents with the same seed must produce identical child streams.
	parentA := New(7, 7)
	parentB := New(7, 7)
	childA := parentA.Split()
	childB := parentB.Split()
	for i := 0; i < 16; i++ {
		if x, y := childA.Float64(), childB.Float64(); x != y {
			t.Fatalf("draw %d: split children from equal-seed parents diverged: %v != %v", i, x, y)
		}
	}
}

func TestSplit_IndependentOfParentAndSiblings(t *testing.T) {
	parent := New(123, 456)
	child1 := parent.Split()
	// Parent must keep advancing independently of its child.
	parentSeq := make([]float64, 8)
	for i := range parentSeq {
		parentSeq[i] = parent.Float64()
	}
	child1Seq := make([]float64, 8)
	for i := range child1Seq {
		child1Seq[i] = child1.Float64()
	}
	if reflect.DeepEqual(parentSeq, child1Seq) {
		t.Fatalf("child stream equals parent stream; Split did not derive an independent stream")
	}

	// A second split off the SAME parent (after it advanced) is a distinct stream.
	child2 := parent.Split()
	child2Seq := make([]float64, 8)
	for i := range child2Seq {
		child2Seq[i] = child2.Float64()
	}
	if reflect.DeepEqual(child1Seq, child2Seq) {
		t.Fatalf("two sibling splits produced identical streams; Split is not deriving fresh seeds")
	}
}
