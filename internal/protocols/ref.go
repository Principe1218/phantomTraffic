package protocols

// Ref is the protocol-agnostic identity of a planned action: which protocol and
// which verb. The behavior layer plans actions as Refs (plus an opaque Params)
// and never touches a concrete action struct, so credentials and bodies cannot
// leak upward (design §3). Stats and logs use Ref.String() as a low-cardinality
// correlation label; it never contains a secret.
type Ref struct {
	Protocol ProtocolID
	Verb     ActionKind
}

// String renders the stable "protocol:verb" label (e.g. "http:fetch-page").
func (r Ref) String() string { return string(r.Protocol) + ":" + string(r.Verb) }

// Params is the opaque, already-validated parameter payload a PlannedAction
// carries. It is an ALIAS of the open Action marker (design §2): the behavior
// layer holds it WITHOUT introspecting it (never reads its fields or calls its
// methods), so bodies/credentials cannot leak upward, yet concrete per-protocol
// param structs in their own subpackages (handler plans 5–8, in- or out-of-tree)
// satisfy it with NO core rebuild. It is deliberately NOT sealed with an
// unexported method, which would force every concrete param type into this
// package. In Plan 3 no concrete actions exist, so a PlannedAction always carries
// Params == nil.
type Params = Action
