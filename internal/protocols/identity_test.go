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
