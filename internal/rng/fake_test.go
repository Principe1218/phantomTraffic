package rng

import (
	"reflect"
	"testing"
)

func TestNewFake_DrivesScriptedSequencesInOrder(t *testing.T) {
	f := NewFake(FakeScript{
		Floats: []float64{0.1, 0.2, 0.3},
		Norms:  []float64{-1.5, 0.0, 2.5},
		Exps:   []float64{0.01, 5.0},
		Ints:   []int{3, 7},
		Int63s: []int64{1000, 2000},
		Perms:  [][]int{{0, 2, 1}},
	})

	if got := []float64{f.Float64(), f.Float64(), f.Float64()}; !reflect.DeepEqual(got, []float64{0.1, 0.2, 0.3}) {
		t.Fatalf("Float64 script: got %v", got)
	}
	if got := []float64{f.NormFloat64(), f.NormFloat64(), f.NormFloat64()}; !reflect.DeepEqual(got, []float64{-1.5, 0.0, 2.5}) {
		t.Fatalf("NormFloat64 script: got %v", got)
	}
	if got := []float64{f.ExpFloat64(), f.ExpFloat64()}; !reflect.DeepEqual(got, []float64{0.01, 5.0}) {
		t.Fatalf("ExpFloat64 script: got %v", got)
	}
	if got := []int{f.IntN(99), f.IntN(99)}; !reflect.DeepEqual(got, []int{3, 7}) {
		t.Fatalf("IntN script: got %v", got)
	}
	if got := []int64{f.Int63n(9999), f.Int63n(9999)}; !reflect.DeepEqual(got, []int64{1000, 2000}) {
		t.Fatalf("Int63n script: got %v", got)
	}
	if got := f.Perm(3); !reflect.DeepEqual(got, []int{0, 2, 1}) {
		t.Fatalf("Perm script: got %v", got)
	}
}

func TestNewFake_PanicsWhenScriptExhausted(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when Float64 script is exhausted, got none")
		}
	}()
	f := NewFake(FakeScript{Floats: []float64{0.5}})
	_ = f.Float64() // consumes the only value
	_ = f.Float64() // must panic: a test out-drawing its script is a test bug
}

func TestFake_SplitIsIndependentlyScripted(t *testing.T) {
	parent := NewFake(FakeScript{Floats: []float64{0.9}})
	child := parent.Split()
	// Consuming the child must not consume the parent's script.
	if got := child.Float64(); got != 0.9 {
		t.Fatalf("child Float64 = %v, want copied 0.9", got)
	}
	if got := parent.Float64(); got != 0.9 {
		t.Fatalf("parent Float64 = %v, want untouched 0.9 (child must hold a deep copy)", got)
	}
}
