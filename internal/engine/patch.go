package engine

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/safety"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

const (
	applyPatchOp = "Run.ApplyPatch"
	targetsAddOp = "engine.patch.targets_add"
)

// MixWeights maps a block ID to its new relative weight. Re-normalized on apply;
// takes effect at the next vuser recycle (foundation §6.7).
type MixWeights map[string]uint

// TargetSpec is a single target to append to a block's frozen allowlist via the
// same scenario.Validate path. Addr is validated against AllowedDomains on apply.
type TargetSpec struct {
	BlockID string
	Addr    string
}

// ScenarioPatch is a bounded, re-validated, audited live modification of a Run.
// All fields are optional; a nil pointer / empty slice means "leave unchanged".
type ScenarioPatch struct {
	Caps           *safety.CapPatch // DOWN free; UP only under the run's cap-override flag
	Concurrency    *int             // bounded; applied through the resizable semaphore
	Weights        *MixWeights      // re-normalized; effective at next vuser recycle
	RotationIntSec *int             // per-block rotation interval, in seconds
	TargetsAdd     []TargetSpec     // extend the frozen allowlist via the SAME Validate path
	TargetsDisable []string         // soft-disable (breaker force-open); never silent removal
}

// ApplyPatch applies a bounded, re-validated, audited live modification to the run.
// Every branch runs under r.patchMu. Caps may be lowered freely; raising a cap
// requires the run's cap-override flag (and emits ActionCapOverrideEnabled). On any
// rejection nothing is mutated and no ActionScenarioPatched is emitted.
func (r *Run) ApplyPatch(_ context.Context, p ScenarioPatch) error {
	r.patchMu.Lock()
	defer r.patchMu.Unlock()

	detail := map[string]string{}

	if p.Caps != nil {
		if err := r.patchCaps(*p.Caps, detail); err != nil {
			return err
		}
	}
	if p.Concurrency != nil {
		if err := r.patchConcurrency(*p.Concurrency, detail); err != nil {
			return err
		}
	}
	if p.Weights != nil {
		if err := r.patchWeights(*p.Weights, detail); err != nil {
			return err
		}
	}
	if p.RotationIntSec != nil {
		if err := r.patchRotation(*p.RotationIntSec, detail); err != nil {
			return err
		}
	}
	if err := r.patchTargetsAdd(p.TargetsAdd, detail); err != nil {
		return err
	}
	if err := r.patchTargetsDisable(p.TargetsDisable, detail); err != nil {
		return err
	}

	return r.finishPatch(detail)
}

func (r *Run) patchCaps(cp safety.CapPatch, detail map[string]string) error {
	next := r.caps.WithPatch(cp)
	if capsRaised(r.caps, next) {
		if !r.capOverride {
			return pterr.New(pterr.ClassConfig, "engine.patch.caps_up",
				applyPatchOp, "raising a safety cap requires the cap-override flag")
		}
		if err := r.audit.Append(audit.Event{
			Actor:    r.AgentID(),
			Action:   audit.ActionCapOverrideEnabled,
			Resource: r.ID(),
			Detail:   map[string]string{"run_id": r.ID()},
		}); err != nil {
			return err
		}
	}
	r.caps = next
	detail["caps"] = "patched"
	return nil
}

func (r *Run) patchConcurrency(n int, detail map[string]string) error {
	if n <= 0 {
		return pterr.New(pterr.ClassConfig, "engine.patch.concurrency",
			applyPatchOp, "concurrency must be >= 1")
	}
	r.sem.setLimit(n)
	detail["concurrency"] = strconv.Itoa(n)
	return nil
}

func (r *Run) patchWeights(w MixWeights, detail map[string]string) error {
	norm, err := r.normalizeWeights(w)
	if err != nil {
		return err
	}
	r.weights = norm
	detail["weights"] = "patched"
	return nil
}

func (r *Run) patchRotation(sec int, detail map[string]string) error {
	if sec < 0 {
		return pterr.New(pterr.ClassConfig, "engine.patch.rotation",
			applyPatchOp, "rotation interval seconds must be >= 0")
	}
	r.selector.setInterval(time.Duration(sec) * time.Second)
	detail["rotation_interval_sec"] = strconv.Itoa(sec)
	return nil
}

func (r *Run) patchTargetsAdd(specs []TargetSpec, detail map[string]string) error {
	for _, spec := range specs {
		blk, ok := r.blockByID(spec.BlockID)
		if !ok {
			return pterr.New(pterr.ClassConfig, targetsAddOp,
				applyPatchOp, "TargetsAdd references unknown block: "+spec.BlockID)
		}
		if err := structuralCheck(blk.Protocol, spec.Addr); err != nil {
			return err
		}
		host, err := hostOf(spec.Addr)
		if err != nil {
			return pterr.New(pterr.ClassConfig, targetsAddOp,
				applyPatchOp, "TargetsAdd invalid address: "+spec.Addr)
		}
		if !r.sc.Targets.Permits(host) {
			return pterr.New(pterr.ClassConfig, targetsAddOp,
				applyPatchOp, "TargetsAdd address not in allowed_domains: "+spec.Addr)
		}
		tid := blk.ID + "/" + strconv.Itoa(len(blk.Targets)+r.addedTargets(blk.ID))
		r.selector.addTarget(blk.Protocol, protocols.Target{ID: tid, Proto: blk.Protocol, Addr: spec.Addr})
		r.coll.addShard(tid)
		r.breakers[tid] = safety.NewBreaker(r.engine.opts.Clock, breakerThreshold, breakerCooldown)
		detail["targets_add"] = detail["targets_add"] + tid + " "
	}
	return nil
}

func (r *Run) patchTargetsDisable(tids []string, detail map[string]string) error {
	for _, tid := range tids {
		b, ok := r.breakers[tid]
		if !ok {
			return pterr.New(pterr.ClassConfig, "engine.patch.targets_disable",
				applyPatchOp, "TargetsDisable references unknown target: "+tid)
		}
		b.ForceOpen()
		detail["targets_disable"] = detail["targets_disable"] + tid + " "
	}
	return nil
}

// capsRaised reports whether next loosens any cap relative to cur.
func capsRaised(cur, next safety.CapSpec) bool {
	return next.PerTargetRPS > cur.PerTargetRPS ||
		next.GlobalRPS > cur.GlobalRPS ||
		next.TotalRequestBudget > cur.TotalRequestBudget
}

// finishPatch emits the mandatory ActionScenarioPatched audit event.
func (r *Run) finishPatch(detail map[string]string) error {
	detail["run_id"] = r.ID()
	return r.audit.Append(audit.Event{
		Actor:    r.AgentID(),
		Action:   audit.ActionScenarioPatched,
		Resource: r.ID(),
		Detail:   detail,
	})
}

// normalizeWeights validates block IDs and GCD-reduces the weight set.
func (r *Run) normalizeWeights(w MixWeights) (map[string]uint, error) {
	known := make(map[string]bool, len(r.weights))
	for id := range r.weights {
		known[id] = true
	}
	out := make(map[string]uint, len(r.weights))
	for id, v := range w {
		if !known[id] {
			return nil, pterr.New(pterr.ClassConfig, "engine.patch.weights",
				applyPatchOp, "weight references unknown block: "+id)
		}
		if v == 0 {
			return nil, pterr.New(pterr.ClassConfig, "engine.patch.weights",
				applyPatchOp, "weight must be >= 1 for block: "+id)
		}
		out[id] = v
	}
	for id, v := range r.weights {
		if _, ok := out[id]; !ok {
			out[id] = v
		}
	}
	g := uint(0)
	for _, v := range out {
		g = gcd(g, v)
	}
	if g > 1 {
		for id := range out {
			out[id] /= g
		}
	}
	return out, nil
}

func gcd(a, b uint) uint {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// blockByID returns the frozen block matching id.
func (r *Run) blockByID(id string) (scenario.Block, bool) {
	for _, b := range r.sc.Blocks {
		if b.ID == id {
			return b, true
		}
	}
	return scenario.Block{}, false
}

// addedTargets counts shards already appended for a block via prior patches.
func (r *Run) addedTargets(blockID string) int {
	n := 0
	for _, id := range r.coll.targetIDs {
		if len(id) > len(blockID)+1 && id[:len(blockID)] == blockID && id[len(blockID)] == '/' {
			n++
		}
	}
	blk, _ := r.blockByID(blockID)
	added := n - len(blk.Targets)
	if added < 0 {
		return 0
	}
	return added
}

// hostOf extracts the host from a target address.
func hostOf(addr string) (string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	if u.Host != "" {
		return u.Hostname(), nil
	}
	return addr, nil
}

// structuralCheck rejects a TargetsAdd whose URL scheme implies a different
// protocol handler than the block already uses (a structural change, not a tune).
func structuralCheck(proto protocols.ProtocolID, addr string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return pterr.New(pterr.ClassConfig, "engine.patch.structural",
			applyPatchOp, "TargetsAdd invalid address: "+addr)
	}
	if u.Scheme == "" {
		return nil // bare host inherits block protocol
	}
	want := schemeFor(proto)
	if u.Scheme != want {
		return pterr.New(pterr.ClassConfig, "engine.patch.structural",
			applyPatchOp,
			"TargetsAdd protocol scheme "+u.Scheme+" does not match block protocol "+string(proto))
	}
	return nil
}

// schemeFor maps a ProtocolID to its accepted URL scheme.
func schemeFor(p protocols.ProtocolID) string {
	switch p {
	case protocols.ProtoHTTP:
		return "https"
	case protocols.ProtoSSH:
		return "ssh"
	case protocols.ProtoDNS:
		return "dns"
	case protocols.ProtoStream:
		return "stream"
	default:
		return string(p)
	}
}
