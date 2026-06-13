package protocols

// NOTE: this file uses the identifier OutcomeCancelled and its string form
// "cancelled" verbatim from the foundation design/plan (§2). These are
// load-bearing contract names other modules match exactly, so the en-GB
// spelling is intentional and required here. en-GB-ok

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

func TestResult_JSONRoundTrip(t *testing.T) {
	in := Result{
		Protocol:  "http",
		Action:    "fetch-page",
		Target:    "web-prod",
		Session:   "sess-abc",
		Seq:       42,
		Nav:       "nav-1",
		Outcome:   OutcomeSuccess,
		ErrClass:  pterr.ClassPermanent, // non-zero so omitempty does not drop it (reviewer note #2)
		ErrCode:   "http.5xx",
		StartedAt: time.Unix(1718000000, 0).UTC(),
		Latency:   125 * time.Millisecond,
		BytesIn:   2048,
		BytesOut:  512,
		HTTP:      &HTTPMeta{Status: 503, Method: "GET", Redirects: 1},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Result must be JSON-serializable: %v", err)
	}
	var out Result
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Result must JSON-round-trip: %v", err)
	}
	if out.HTTP == nil || out.HTTP.Status != 503 {
		t.Fatalf("HTTP meta lost in round-trip: %+v", out.HTTP)
	}
	if out.Seq != 42 || out.ErrCode != "http.5xx" || out.BytesIn != 2048 {
		t.Fatalf("scalar fields lost in round-trip: %+v", out)
	}
	if out.ErrClass != pterr.ClassPermanent {
		t.Fatalf("ErrClass lost in round-trip: got %v, want %v", out.ErrClass, pterr.ClassPermanent)
	}
}

// TestResult_NoSecretFields is a structural guard (AGENTS.md §3.1, §5.5):
// Result is the SCRUBBED envelope that reaches logs/UI/stats, so no field
// name may suggest it carries a secret, header, body, or stack.
func TestResult_NoSecretFields(t *testing.T) {
	banned := []string{
		"secret", "password", "passwd", "token", "credential", "cred",
		"authorization", "cookie", "header", "body", "stack", "key", "stdout",
	}
	walkFieldNames(t, reflect.TypeOf(Result{}), banned, "Result")
}

func walkFieldNames(t *testing.T, typ reflect.Type, banned []string, path string) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		lower := strings.ToLower(f.Name)
		for _, b := range banned {
			if strings.Contains(lower, b) {
				t.Errorf("%s.%s field name suggests a secret/raw payload (banned substring %q)", path, f.Name, b)
			}
		}
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct && ft.PkgPath() == typ.PkgPath() {
			walkFieldNames(t, ft, banned, path+"."+f.Name)
		}
	}
}

func TestOutcome_String(t *testing.T) {
	cases := map[Outcome]string{
		OutcomeSuccess:   "success",
		OutcomeFailure:   "failure",
		OutcomeSkipped:   "skipped",
		OutcomeCancelled: "cancelled",
		OutcomePanicked:  "panicked",
		OutcomeReconnect: "reconnect",
	}
	for o, want := range cases {
		if got := o.String(); got != want {
			t.Errorf("Outcome(%d).String() = %q, want %q", o, got, want)
		}
	}
	if got := Outcome(99).String(); got != "outcome(99)" {
		t.Errorf("unknown Outcome String = %q, want outcome(99)", got)
	}
}
