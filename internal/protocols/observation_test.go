package protocols

import (
	"reflect"
	"testing"
	"time"
)

// TestObservation_ByValueNoHandlerPointers enforces the design §2 lifetime
// contract: Observation is passed BY VALUE and carries NO pointers into
// handler-owned memory. We allow only value types, bounded scalars, slices
// of scalars/strings, and fixed-size arrays. No pointer/chan/func/map/
// unsafe fields anywhere in the transitive structure.
func TestObservation_ByValueNoHandlerPointers(t *testing.T) {
	assertNoReferenceFields(t, reflect.TypeOf(Observation{}), "Observation")
}

func assertNoReferenceFields(t *testing.T, typ reflect.Type, path string) {
	t.Helper()
	switch typ.Kind() {
	case reflect.Ptr, reflect.Chan, reflect.Func, reflect.Map, reflect.UnsafePointer, reflect.Interface:
		t.Errorf("%s has reference kind %s — Observation must contain no aliasing into handler memory", path, typ.Kind())
		return
	case reflect.Slice, reflect.Array:
		assertNoReferenceFields(t, typ.Elem(), path+"[]")
		return
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			assertNoReferenceFields(t, f.Type, path+"."+f.Name)
		}
	}
}

func TestObsMask_BitsIndependent(t *testing.T) {
	var m ObsMask
	if m.Has(ObsDNS) || m.Has(ObsHTTP) || m.Has(ObsSSH) || m.Has(ObsStream) {
		t.Fatal("zero ObsMask must report no populated sub-structs")
	}
	m = m.Set(ObsDNS).Set(ObsStream)
	if !m.Has(ObsDNS) {
		t.Error("ObsDNS bit should be set")
	}
	if !m.Has(ObsStream) {
		t.Error("ObsStream bit should be set")
	}
	if m.Has(ObsHTTP) || m.Has(ObsSSH) {
		t.Error("unset bits must report false")
	}
}

func TestObservation_FieldsHoldValues(t *testing.T) {
	obs := Observation{
		Throughput:   1_500_000,
		BufferHealth: 8 * time.Second,
		DNS:          DNSObs{TTLSeconds: 300, Rcode: "NOERROR", RecordTypes: []string{"A", "AAAA"}},
		SSH:          SSHObs{ExitCode: 0, StdoutLen: 64, StdoutPrefix: "total 8"},
		Has:          ObsMask(0).Set(ObsDNS).Set(ObsSSH),
	}
	if obs.DNS.TTLSeconds != 300 || len(obs.DNS.RecordTypes) != 2 {
		t.Fatalf("DNSObs not carried by value: %+v", obs.DNS)
	}
	if obs.SSH.StdoutPrefix != "total 8" {
		t.Fatalf("SSHObs prefix not carried: %+v", obs.SSH)
	}
	if !obs.Has.Has(ObsDNS) || !obs.Has.Has(ObsSSH) {
		t.Fatal("Has mask not threaded through")
	}
}
