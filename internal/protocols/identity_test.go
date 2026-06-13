package protocols

import "testing"

func TestIsKnownProtocol(t *testing.T) {
	cases := []struct {
		name string
		in   ProtocolID
		want bool
	}{
		{name: "http is known", in: ProtoHTTP, want: true},
		{name: "dns is known", in: ProtoDNS, want: true},
		{name: "ssh is known", in: ProtoSSH, want: true},
		{name: "stream is known", in: ProtoStream, want: true},
		{name: "http literal is known", in: ProtocolID("http"), want: true},
		{name: "dns literal is known", in: ProtocolID("dns"), want: true},
		{name: "ssh literal is known", in: ProtocolID("ssh"), want: true},
		{name: "stream literal is known", in: ProtocolID("stream"), want: true},
		{name: "gopher is unknown", in: ProtocolID("gopher"), want: false},
		{name: "empty is unknown", in: ProtocolID(""), want: false},
		{name: "uppercase HTTP is unknown", in: ProtocolID("HTTP"), want: false},
		{name: "mixed-case Stream is unknown", in: ProtocolID("Stream"), want: false},
		{name: "padded http is unknown", in: ProtocolID(" http"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsKnownProtocol(tc.in); got != tc.want {
				t.Errorf("IsKnownProtocol(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestKnownProtocols_StableSortedOrder(t *testing.T) {
	want := []ProtocolID{ProtoDNS, ProtoHTTP, ProtoSSH, ProtoStream}
	got := KnownProtocols()
	if len(got) != len(want) {
		t.Fatalf("KnownProtocols() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("KnownProtocols()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestKnownProtocols_AllocationIndependence(t *testing.T) {
	first := KnownProtocols()
	// Mutate the returned slice; this must not leak into the package state.
	for i := range first {
		first[i] = ProtocolID("tampered")
	}
	second := KnownProtocols()
	want := []ProtocolID{ProtoDNS, ProtoHTTP, ProtoSSH, ProtoStream}
	if len(second) != len(want) {
		t.Fatalf("second call len = %d, want %d (%v)", len(second), len(want), second)
	}
	for i := range want {
		if second[i] != want[i] {
			t.Errorf("after mutating first call, KnownProtocols()[%d] = %q, want %q", i, second[i], want[i])
		}
	}
}
