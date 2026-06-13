package protocols

import "testing"

// twoActions: two distinct concrete Action types embedding BaseAction, used
// purely to exercise As[T] success and *RoutingError on a wrong cast. No I/O.
type fakeReqAction struct {
	BaseAction
	Path string
}

func (a fakeReqAction) Kind() ActionKind { return "request" }
func (a fakeReqAction) Validate() error  { return nil }

type fakeQueryAction struct {
	BaseAction
	QName string
}

func (a fakeQueryAction) Kind() ActionKind { return "query" }
func (a fakeQueryAction) Validate() error  { return nil }

// compile-time proof both satisfy Action (mirrors the §3 var-assertion idiom).
var _ Action = fakeReqAction{}
var _ Action = fakeQueryAction{}

func TestBaseAction_Accessors(t *testing.T) {
	a := fakeReqAction{BaseAction: BaseAction{Proto: "http", C: CauseNavigation, P: PacingShaperManaged}}
	if a.Protocol() != "http" {
		t.Fatalf("Protocol() = %q, want http", a.Protocol())
	}
	if a.Cause() != CauseNavigation {
		t.Fatalf("Cause() = %v, want CauseNavigation", a.Cause())
	}
	if a.Pacing() != PacingShaperManaged {
		t.Fatalf("Pacing() = %v, want PacingShaperManaged", a.Pacing())
	}
	if a.Kind() != "request" {
		t.Fatalf("Kind() = %q, want request", a.Kind())
	}
}

func TestAs_Success(t *testing.T) {
	var a Action = fakeReqAction{BaseAction: BaseAction{Proto: "http"}, Path: "/index.html"}
	got, err := As[fakeReqAction](a)
	if err != nil {
		t.Fatalf("As succeed expected, got err: %v", err)
	}
	if got.Path != "/index.html" {
		t.Fatalf("As recovered Path = %q, want /index.html", got.Path)
	}
}

func TestAs_WrongType_ReturnsRoutingError(t *testing.T) {
	var a Action = fakeQueryAction{BaseAction: BaseAction{Proto: "dns"}, QName: "example.com"}
	_, err := As[fakeReqAction](a)
	if err == nil {
		t.Fatal("As must error when the concrete type does not match")
	}
	re, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("As must return *RoutingError, got %T", err)
	}
	if re.Got != "query" {
		t.Fatalf("RoutingError.Got = %q, want query", re.Got)
	}
}

func TestAs_NilAction_ReturnsRoutingErrorWithoutPanic(t *testing.T) {
	// A nil Action reaching the audited cast site must NOT panic (the "never
	// panics" contract is absolute); it returns a *RoutingError instead.
	var a Action // nil interface
	got, err := As[fakeReqAction](a)
	if err == nil {
		t.Fatal("As(nil) must return an error, not the zero value with nil err")
	}
	if _, ok := err.(*RoutingError); !ok {
		t.Fatalf("As(nil) must return *RoutingError, got %T", err)
	}
	if got != (fakeReqAction{}) {
		t.Fatalf("As(nil) must return the zero T, got %+v", got)
	}
}

func TestCause_String(t *testing.T) {
	cases := map[Cause]string{
		CauseNavigation:  "navigation",
		CauseSubResource: "sub-resource",
		CauseBackground:  "background",
		CauseControl:     "control",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("Cause(%d).String() = %q, want %q", c, got, want)
		}
	}
	if got := Cause(200).String(); got != "cause(200)" {
		t.Errorf("unknown Cause String = %q, want cause(200)", got)
	}
}

func TestPacingMode_String(t *testing.T) {
	if got := PacingShaperManaged.String(); got != "shaper-managed" {
		t.Errorf("PacingShaperManaged.String() = %q", got)
	}
	if got := PacingSelfPaced.String(); got != "self-paced" {
		t.Errorf("PacingSelfPaced.String() = %q", got)
	}
	if got := PacingMode(9).String(); got != "pacing(9)" {
		t.Errorf("unknown PacingMode String = %q, want pacing(9)", got)
	}
}
