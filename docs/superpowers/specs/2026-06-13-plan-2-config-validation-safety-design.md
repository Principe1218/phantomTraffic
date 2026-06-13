<!-- DRAFT — Plan 2 scope/design. Scopes the "Config + validation + safety caps" phase
against the ratified core foundation design (2026-06-11-core-foundation-design.md). Local-only
(docs/superpowers/ is gitignored). Pending maintainer review before writing-plans. -->

# PhantomTraffic — Plan 2 Scope & Design: Config, Validation, Safety Caps

**Date:** 2026-06-13
**Status:** Draft — pending maintainer review, then `writing-plans`
**Builds on:** Plan 1 (Foundation, merged to `main`)
**Design authority:** [`2026-06-11-core-foundation-design.md`](2026-06-11-core-foundation-design.md)
(§5.2 parse→validate→freeze, §6 safety caps, **D2** cap ceilings, **D4** insecure opt-in)

---

## 1. Goal

Make `phantom validate scenario.yaml` a real, runnable command that turns an untrusted
scenario file into an immutable, fully-validated `Scenario` — **or** a clear, actionable
list of errors. **No network. No traffic. No execution.** This is the static, security-
critical front door every later plan (behavior, engine, handlers) consumes.

> "An invalid scenario is unrepresentable past the validation boundary." (design §5.2)

---

## 2. Scope boundary (the four ratified decisions)

| Decision | Resolution |
| --- | --- |
| **Schema scope** | **Incremental vertical slice.** Define + validate only what Plan 2 can fully exercise (structure, targeting, caps, security posture, execution mode). Deep per-protocol action schemas and behavior/persona/ramp/schedule fields grow in their own plans. Strict decoding rejects unknown keys until then. |
| **Validate depth** | **Static only.** Schema, enums, numeric bounds, allowlist construction, cap ceilings, insecure-gating, cross-references. No DNS resolution, no TCP dial, no credential-file reads. `--reachability` and SSH key/known_hosts checks are deferred to their handler plans. |
| **Safety caps** | **Config-time only.** Compiled-in ceiling constants + declared-cap config types + caps-down-free / caps-up-override-gated validation + the override flag + `--agent-count` guardrail. The runtime token-bucket `Limiter`/`Reservation`/tripwire/breakers stay in Plan 4 (Engine + safety). |
| **CLI** | **stdlib `flag`, zero new runtime deps for the CLI layer.** Hand-rolled subcommand dispatch; only `phantom validate` exists now. Cobra deliberately deferred to Plan 9 (CLI consolidation), revisited then with a `§7.1` dossier. |

### Carryover invariants honored from the foundation design

- Two-stage **parse → validate → freeze**: `Raw*` decode structs → `Validate()` →
  immutable `Scenario` with typed `Protocol` enum and pre-parsed targets (§5.2).
- The **authoritative allowlist** is built here: `Validate` constructs the frozen
  `protocols.TargetSet` via the existing `protocols.NewTargetSet` (Plan 1).
- **Dual-gate insecure opt-in (D4):** invocation `--allow-insecure` **and** scenario
  `allow_insecure: true` + non-empty reason. Plan 2 validates the gate at **scenario
  level**; the per-target override (D4) lands with the handler plans, when targets gain
  protocol-specific structure.
- **Validation is pure / side-effect-free.** Audit events for cap-override and insecure
  transport are emitted at *run* time (Plan 4 / handlers), not at validate time. Plan 2
  `validate` reads and reports; it never writes an audit record or opens a socket.

---

## 3. Package layout

```text
cmd/phantom/
  main.go            # thin: parse args → dispatch subcommand → os.Exit(code)
  validate.go        # runValidate(stdout, stderr io.Writer, args []string) int  (testable; cobra-swappable)
  validate_test.go

internal/scenario/   # "scenario file → validated, frozen Scenario" (one cohesive flow)
  raw.go             # Raw decode structs (yaml-tagged); strict decode (KnownFields)
  scenario.go        # immutable Scenario domain type + typed Protocol enum + accessors
  validate.go        # Validate(Raw, Options) (Scenario, error) — pure; aggregates field errors
  load.go            # Load(path) (Raw, error) — file read + size cap + strict decode (owns the YAML dep)
  *_test.go

internal/safety/     # EXTENDED (Plan 1 left Caps as a stub)
  ceiling.go         # compiled-in hard maxima (D2 constants) + effective-ceiling-for-N-agents
  capspec.go         # declared scenario caps config type (the full D2 cap set)
  validate.go        # ValidateCaps(declared, ceiling, override) error  (caps-down free / caps-up gated)
  *_test.go

internal/protocols/  # ADDITIVE touch to Plan 1: canonical ProtocolID constants the
  identity.go        #   schema validates against (ProtoHTTP/ProtoDNS/ProtoSSH/ProtoStream)
  identity_test.go   #   + IsKnownProtocol(ProtocolID) bool / KnownProtocols() helper.
```

The `internal/protocols` change is additive and small — it gives the canonical closed set
of protocol ids a single authoritative home (the design defines them as `"http"`, `"dns"`,
`"ssh"`, `"stream"`). Validation needs this because the runtime `Registry` is empty until
handlers register in Plan 5+.

**Why one `internal/scenario` package, not a `config` + `scenario` split** (the design's
project-structure listing names both): for the current single-file, single-format flow,
splitting "load YAML" from "validate" is premature. `load.go` already isolates the I/O +
YAML-library surface inside the package, and `validate.go` is a pure function with no I/O,
so the two concerns are cleanly separable *within* one package and independently testable.
We extract `internal/config` only if/when loading grows (multiple formats, includes,
env-overlays). Flagged for your review (§11).

> The existing `internal/safety/caps.go` stub (`Caps{MaxConcurrent, MaxRPS,
> PerActionLimit}`) is the *effective per-session merged* set carried on `SessionDeps`;
> the merge happens at engine session-construction in Plan 4. Plan 2 leaves it in place
> and **adds** the declared-cap + ceiling + validation types alongside it.

---

## 4. Scenario schema — the Plan 2 slice

Top-level and per-block fields Plan 2 **defines and validates** (strict decode, unknown
keys rejected):

```yaml
name: "Office hours traffic"            # required, bounded length
description: "..."                      # optional, bounded length
allowed_domains: ["cdn.company.com"]    # optional; admitted to the allowlist, host-validated
# agent_count is NOT a YAML field — it is the invocation-time `--agent-count N` CLI flag
# (D3 guardrail; divides cap ceilings). An operator concern, not a scenario property.

caps:                                   # optional; see §6. Omitted ⇒ compiled-in ceilings apply.
  per_target_rps: 5
  global_rps: 25
  max_concurrent_sessions: 10
  total_request_budget: 50000
  streaming_byte_rate_kbps: 8000
  concurrent_streams: 2
  per_session_max_duration_seconds: 1800
  per_session_max_actions: 10000

execution:
  mode: "parallel"                      # enum: parallel | sequential
  stop_on_error: false

scenarios:                              # ≥1; the per-protocol target-and-action-source groups
  - id: "web-browsing"                  # required; unique within file
    protocol: "http"                    # enum: must be a known protocols.ProtocolID
    targets: ["app1.company.com:443"]   # ≥1; host[:port] validated; no embedded creds
    target_rotation: "sequential"       # enum: random | sequential
    target_rotation_interval_seconds: 300   # >= 0 (0 = engine default)
    allow_insecure: false               # D4 scenario-level default (optional)
    allow_insecure_reason: ""           # required non-empty IFF allow_insecure: true
```

**Explicitly deferred to later plans** (NOT in the Plan 2 schema; a scenario using them
will not validate until its plan extends the decode structs — the honest consequence of
strict decoding + the incremental slice):

- Per-protocol **action sources**: HTTP `requests`, DNS `queries`, SSH `commands`,
  streaming manifests → land with each handler plan (5–8).
- **Persona / behavior**: `TemplateMix`, think-time/jitter/burst distributions,
  fingerprints → Plan 3 (behavior) / persona work.
- **Ramp & schedule**: `RampPlan`, time-of-day windows, `WeightBasis` weights → engine
  plan (4) and scheduling.
- **Per-target insecure override** and **credential references** (`key_path` →
  `secret.CredentialRef`): require targets to become protocol-specific objects → land
  with the handler plans (6–7). Plan 2 targets are validated address strings with no
  credential bytes (consistent with "static only — no credential-file reads").

---

## 5. Validation rules (maps to design §5.2 + AGENTS.md §5.2)

All failures classify as `pterr.ClassConfig` and **fail fast at `validate`**. Errors are
*aggregated* (report every problem in one pass, not just the first), each with a field
path + message.

- **Strict decode:** `KnownFields(true)` — unknown/typo'd keys rejected loudly (e.g. a
  typo'd `allow_inscure` fails, never silently defaults insecure).
- **Required:** `name`; each block's `id`, `protocol`, ≥1 `target`; ≥1 scenario block.
- **Enums (allowlists):** `protocol` ∈ the canonical closed `ProtocolID` set
  {`http`,`dns`,`ssh`,`stream`} — compile-time constants, **not** the runtime registry
  (which is empty until handlers land in Plan 5+); `target_rotation` ∈
  {`random`,`sequential`}; `execution.mode` ∈ {`parallel`,`sequential`}.
- **Targets:** RFC-1035 hostname charset + length; port ∈ 1–65535; **reject embedded URL
  credentials** (`url.User != nil`); parse to a validated address form (never a raw user
  string toward a dialer downstream). Build the frozen `TargetSet`.
- **Uniqueness:** scenario `id` unique within the file.
- **Numeric bounds:** `target_rotation_interval_seconds` ≥ 0 (0 = engine default);
  `agent_count` ≥ 1; all caps ≥ 0 and ≤ effective ceiling (unless override — §6); bounded
  `name`/`description` length.
- **Insecure gating (D4):** if scenario `allow_insecure: true` → require non-empty reason
  **and** require invocation `--allow-insecure`; otherwise `ClassConfig`. (Per-target
  override deferred — §10.) A scenario file alone can never unlock insecure transport.

Output of a successful validate: an immutable `scenario.Scenario` (typed `Protocol`,
pre-parsed targets, frozen `TargetSet`, validated caps) — proof the boundary holds.

---

## 6. Safety caps — config-time model (D2)

**Compiled-in hard ceilings** (`internal/safety/ceiling.go`) — the conservative maxima
config may *lower* freely but *raise* only via the audited override flag. Proposed
defaults (confirm the exact numbers):

| Cap | Proposed default ceiling |
| --- | --- |
| per-target request rate | 10 req/s |
| global request rate | 50 req/s |
| max concurrent sessions | 20 |
| total request budget | finite (e.g. 1,000,000) |
| streaming byte-rate | e.g. 12,000 kbps |
| concurrent streams | e.g. 3 |
| per-session max duration | 30 min |
| per-session max actions | 10,000 |

**Validation:** each declared cap must be ≤ the effective ceiling, where
`effective = ceiling / agent_count` (the D3 distributed guardrail). Declaring any cap
**above** the ceiling requires the named override flag
`--i-understand-this-can-dos-targets`; without it → `ClassConfig`. With it, `validate`
reports that the override is in effect (the *audit record* itself is written at run time,
Plan 4 — validate stays pure). Omitting `caps:` entirely is valid and means "use the
compiled-in ceilings."

---

## 7. CLI surface

```text
phantom validate <file> [--allow-insecure] [--i-understand-this-can-dos-targets]
                        [--agent-count N] [--json]
```

- **Layering:** `main.go` does dispatch only; `runValidate(stdout, stderr, args) int`
  holds the logic and returns an exit code, so it is unit-testable with in-memory buffers
  and swappable for a cobra `RunE` at Plan 9 with no change to the command body.
- **stdlib `flag` quirk, documented:** flags precede the positional file
  (`phantom validate --json scenario.yaml`). Usage string states `[flags] <file>`.
- **Output:** human-readable summary to **stdout** (`✓ scenario.yaml valid — 3 scenarios,
  5 targets`); errors to **stderr** (design §5). `--json` emits
  `{ "valid": bool, "errors": [{"field","message","class"}], "summary": {...} }` for
  piping.
- **Exit codes:** `0` valid; `1` invalid scenario; `2` usage/IO error (file missing, bad
  flags).

---

## 8. Dependencies — one new dep, needs `§7.1` sign-off (security-review trigger)

Plan 2 needs a **strict-decode YAML library** — unknown-key rejection is *security-load-
bearing* (§5.2 / §3): it is what makes a typo'd `allow_inscure` fail loudly instead of
silently defaulting to insecure. stdlib has no YAML.

Design **D1** pre-approved `gopkg.in/yaml.v3` *in principle*, with the actual `§7.1`
dossier due *at add time* — which is now. **Per your "prefer well-maintained libraries"
steer, this needs a real look, because `gopkg.in/yaml.v3`'s upstream was archived.**
Candidates to vet in the dossier (the implementation plan's first task):

| Candidate | Notes (to verify in the dossier at add time) |
| --- | --- |
| `go.yaml.in/yaml/v3` | Community-maintained continuation of the same codebase; drop-in API incl. `KnownFields`. Likely the maintained successor — **recommended pending verification**. |
| `gopkg.in/yaml.v3` | The design's original pick; upstream archived — verify last release / OSV before accepting. |
| `goccy/go-yaml` | Actively maintained, pure-Go, better error messages; different API; weigh past advisories. |

The `§7.1` dossier (version, maintenance recency per §7.2, open CVEs via OSV/GHSA,
license, transitive count, de-facto status) + final selection is **Task 1 of the
implementation plan and gated on your approval** before any code assumes an import.

No other new runtime dependencies. (Plan 1's dev-only `goleak` test harness continues.)

---

## 9. Testing approach (strict TDD, red→green→refactor)

- `internal/scenario`: table-driven decode+validate tests — a golden valid fixture →
  expected frozen `Scenario`; one failing test per invalid case (unknown key, bad
  protocol, bad host, embedded creds, missing insecure reason, insecure without
  `--allow-insecure`, duplicate id, cap over ceiling without override, agent-count
  division). Validation is pure ⇒ no fixtures touch the network or filesystem beyond
  `testdata/` scenario files.
- `internal/safety`: `ValidateCaps` table tests — under / at / over ceiling, override on,
  `agent_count` division, omitted caps.
- `cmd/phantom`: `runValidate` exit-code + stdout/stderr + `--json` shape tests for valid,
  invalid, and the `--allow-insecure` gating, using in-memory writers + temp files.
- `testdata/`: a small library of valid and intentionally-broken scenario fixtures.

---

## 10. Out of scope (deferred, by plan)

- Runtime safety enforcement — `Limiter`, `Reservation`, tripwire, circuit breakers →
  **Plan 4** (Engine + safety).
- `--reachability` pre-flight (DNS resolve, TCP dial) and SSH key-file / `known_hosts`
  checks → **handler plans (6–7)**.
- Per-protocol action schemas, persona/behavior, ramp, schedule, weighted-mix → **Plans
  3–8** as listed in §4.
- Per-target insecure override + credential references (targets-as-objects) → **handler
  plans (6–7)**.
- Cobra CLI framework, `phantom run` / `generate` / `list-protocols` → **Plan 9**.
- Wails UI form validation reusing this `Validate()` → **Plan 10**.

---

## 11. Open decisions for your review

1. **YAML library (blocking, §8):** approve producing the `§7.1` dossier and selecting the
   maintained variant (recommend `go.yaml.in/yaml/v3` pending verification) over the
   archived `gopkg.in/yaml.v3`?
2. **Package layout (§3):** single `internal/scenario` package (recommended) vs. the
   design's `internal/config` + `internal/scenario` split?
3. **Cap ceiling numbers (§6):** accept the proposed default maxima, or adjust any?

---

## 12. Next step

On approval of this scope (and decision #1 especially), invoke **`writing-plans`** to
produce `docs/superpowers/plans/2026-06-13-plan-2-config-validation-safety.md` — a sibling
to the foundation plan, same Module → Task → *Types introduced* → strict-TDD-checkbox
format, in build order: (1) YAML dep dossier + decode, (2) `internal/scenario` schema +
validate, (3) `internal/safety` ceilings + cap validation, (4) `cmd/phantom validate` +
output + exit codes.
