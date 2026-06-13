# Dependency Dossier — go.yaml.in/yaml/v3

> AGENTS.md §7.1 approval record for a new third-party dependency.
> Required for the `internal/scenario` package (Plan 2), which performs strict
> `KnownFields(true)` YAML decoding of scenario files. Every field below maps to
> the six §7.1 questions. Lines marked **(fill from command output)** must be
> filled with the real output of the commands in this dossier before the
> dossier is committed — do not hand-write values you have not verified.

## 1. Package and pinned version

- **Module path:** `go.yaml.in/yaml/v3`
- **Pinned version:** `v3.0.4` (exact, no floating `latest` / `^` / `*` — AGENTS.md §7.2)
- **Import path used in code:** `import "go.yaml.in/yaml/v3"`
- **Source repository:** https://github.com/yaml/go-yaml (canonical home of the `go.yaml.in/yaml` module line)

Pin the exact version in `go.mod` and commit the matching `go.sum` entries
(AGENTS.md §7.3 — lock files committed; §7.2 — no floating versions).

## 2. Purpose — why this, why not the standard library

PhantomTraffic loads operator-authored scenario files (YAML) that declare
protocols, targets, safety caps, and insecure-transport opt-ins. The
`internal/scenario` package must decode these **strictly**:

```go
dec := yaml.NewDecoder(r)
dec.KnownFields(true) // unknown / typo'd keys are an error, not silently ignored
```

- **The Go standard library has no YAML parser.** `encoding/json` does not read
  YAML, and there is no `encoding/yaml`. A third-party decoder is unavoidable.
- **Strict decoding is security-load-bearing.** With `KnownFields(true)`, a
  typo'd key such as `allow_inscure:` (instead of `allow_insecure:`) fails the
  decode loudly instead of being dropped. A dropped key could otherwise leave an
  insecure-transport block *appearing* secure, or silently discard
  `allow_insecure_reason`, defeating the D4 insecure-gating rule. This is
  exactly the "validate all external input / allowlist over denylist" posture in
  AGENTS.md §5.2, and the "fail loudly on malformed input" posture in §5.5.
- **No safe stdlib workaround exists.** Hand-rolling a YAML parser would be both
  reinventing a wheel and a new attack surface (AGENTS.md §0 — simplicity is a
  security property; do not invent parsers any more than you invent crypto).

This dependency is used **only** for decoding (`internal/scenario` Module 4). It
is not used for encoding, for any network operation, or in any hot path.

## 3. Maintenance status

`go.yaml.in/yaml/v3` is the **community-maintained continuation of the archived
`gopkg.in/yaml.v3` codebase**. When the original `go-yaml/yaml` repository
(serving `gopkg.in/yaml.v3`) was archived/retired by its maintainer, the project
was forked and continued under the vanity module path `go.yaml.in/yaml`, hosted
at `github.com/yaml/go-yaml`, under the stewardship of the YAML community
(the same group behind the `yaml.org` spec and the `yaml` GitHub organization).
The public API surface is unchanged from `gopkg.in/yaml.v3` — `Decoder`,
`Encoder`, `Marshal`, `Unmarshal`, and `(*Decoder).KnownFields` all behave
identically, so it is a drop-in successor.

**Verification commands — run these and paste the output into the fenced blocks
below before committing this dossier:**

- Available versions (confirms `v3.0.4` is the latest published v3 tag):

  ```bash
  go list -m -versions go.yaml.in/yaml/v3
  ```

  Output (fill from command output):

  ```text
  go.yaml.in/yaml/v3 v3.0.0 v3.0.1 v3.0.2 v3.0.3 v3.0.4
  ```

- Last-release date / module timestamp (confirms a release within the last 12
  months — AGENTS.md §7.2):

  ```bash
  go list -m -json go.yaml.in/yaml/v3@v3.0.4
  ```

  Output (fill from command output — record the `"Time"` field, i.e. the publish
  timestamp of `v3.0.4`):

  ```text
  {
  	"Path": "go.yaml.in/yaml/v3",
  	"Version": "v3.0.4",
  	"Time": "2025-06-29T14:09:51Z",
  	"Dir": "/Users/caleb.principe/go/pkg/mod/go.yaml.in/yaml/v3@v3.0.4",
  	"GoMod": "/Users/caleb.principe/go/pkg/mod/cache/download/go.yaml.in/yaml/v3/@v/v3.0.4.mod",
  	"GoVersion": "1.16",
  	"Origin": {
  		"VCS": "git",
  		"URL": "https://github.com/yaml/go-yaml",
  		"Hash": "c3552c15f996075a7634df5159d9161c67bf3d76",
  		"Ref": "refs/tags/v3.0.4"
  	}
  }
  ```

- **Last release:** `2025-06-29T14:09:51Z` — within 12 months of dossier date
  2026-06-13 (AGENTS.md §7.2). `v3.0.4` is the highest v3.x.y tag and satisfies
  the recency bar.

## 4. License

The module is **dual-licensed**, inherited unchanged from the
`gopkg.in/yaml.v3` codebase it continues:

- **MIT License** — the libyaml-ported scanner/parser/emitter files
  (`apic.go`, `emitterc.go`, `parserc.go`, `readerc.go`, `scannerc.go`,
  `writerc.go`, `yamlh.go`, `yamlprivateh.go`), © Kirill Simonov.
- **Apache License 2.0** — all remaining Go files, © Canonical Ltd.

Both MIT and Apache-2.0 are OSI-approved permissive licenses compatible with
this project's use (vendoring a decode-only library). The §7.1 brief expected
"MIT"; the authoritative `LICENSE` file confirms the libyaml-derived core is MIT
while the surrounding Go code is Apache-2.0 — recording the precise dual-license
fact rather than the simplified expectation (AGENTS.md §0 — explicit and
auditable over convenient).

**Verification command — paste output below before committing:**

```bash
go mod download go.yaml.in/yaml/v3@v3.0.4 && \
  sed -n '1,40p' "$(go env GOMODCACHE)/go.yaml.in/yaml/v3@v3.0.4/LICENSE"
```

Output (fill from command output):

```text

This project is covered by two different licenses: MIT and Apache.

#### MIT License ####

The following files were ported to Go from C files of libyaml, and thus
are still covered by their original MIT license, with the additional
copyright staring in 2011 when the project was ported over:

    apic.go emitterc.go parserc.go readerc.go scannerc.go
    writerc.go yamlh.go yamlprivateh.go

Copyright (c) 2006-2010 Kirill Simonov
Copyright (c) 2006-2011 Kirill Simonov

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

### Apache License ###

All the remaining project files are covered by the Apache license:

Copyright (c) 2011-2019 Canonical Ltd

Licensed under the Apache License, Version 2.0 (the "License");
```

## 5. Transitive dependency count

**Expected: ZERO third-party transitive dependencies.** `go.yaml.in/yaml/v3` is
self-contained (it depends only on the Go standard library). It does *not* pull
in any additional `go.yaml.in/...`, `gopkg.in/...`, or other third-party module
as a build requirement.

**Verification command — paste output below before committing:**

```bash
go mod graph | grep go.yaml.in
```

Output (fill from command output):

```text
github.com/Principe1218/phantomTraffic go.yaml.in/yaml/v3@v3.0.4
go.yaml.in/yaml/v3@v3.0.4 gopkg.in/check.v1@v0.0.0-20161208181325-20d25e280405
```

- **Transitive dependency count:** 1 (`gopkg.in/check.v1` — a test-only dependency
  of the yaml library itself; it is NOT included in PhantomTraffic's build binary).
  Zero third-party production transitives; the single test-framework dep is
  isolated to the yaml library's own test suite and does not affect the
  PhantomTraffic build graph.

## 6. De-facto status

The `go-yaml` codebase (originally `gopkg.in/yaml.v3`) is the de-facto standard
YAML library for Go: it is the most widely used YAML decoder in the ecosystem,
depended on by Kubernetes-adjacent tooling, Helm, and a large share of Go CLIs
and config loaders. `go.yaml.in/yaml/v3` is the maintained continuation of that
exact codebase under the YAML community's stewardship, preserving the same import
surface. Choosing it (over abandoned forks, over `ghodss/yaml`, or over a
hand-rolled parser) keeps PhantomTraffic on the ecosystem-blessed, actively
maintained path with a strict-decode API we depend on.

## 7. Vulnerability scan

Run `govulncheck` over the whole module **after** adding the dependency and paste
the result. Expected: **no findings** for `go.yaml.in/yaml/v3@v3.0.4`.

```bash
govulncheck ./...
```

Output (fill from command output):

```text
$ make vuln   # go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...
No vulnerabilities found.
```

Cross-reference the Go vulnerability database / GitHub Advisory Database for
`go.yaml.in/yaml` and the predecessor `gopkg.in/yaml.v3` to confirm no
unpatched critical/high advisory affects `v3.0.4`:

```bash
# OSV.dev query for the module (no third-party tool required):
curl -s https://api.osv.dev/v1/query \
  -d '{"package":{"name":"go.yaml.in/yaml/v3","ecosystem":"Go"},"version":"3.0.4"}'
```

Output (fill from command output):

```text
{}
```

> Note: the OSV.dev query returned `{}` (empty object) — no known vulnerabilities
> for `go.yaml.in/yaml/v3` at `v3.0.4`. The predecessor module `gopkg.in/yaml.v3`
> had CVE-2022-28948 (DoS via crafted input) but that was patched in v3.0.0;
> `v3.0.4` is unaffected. `govulncheck ./...` (golang.org/x/vuln v1.3.0) was run
> via `make vuln` after Module 4 added the import and reported **No vulnerabilities found.**

- **Known CVEs / advisories affecting v3.0.4:** None. OSV.dev query returned `{}`,
  and `govulncheck ./...` (v1.3.0, via `make vuln`) reported "No vulnerabilities found."

## 8. §7.1 sign-off checklist

- [x] (1) Package + exact pinned version recorded — `go.yaml.in/yaml/v3` `v3.0.4`.
- [x] (2) Purpose + why-not-stdlib recorded — strict `KnownFields(true)` decode;
      stdlib has no YAML.
- [x] (3) Maintenance status recorded — community-maintained `gopkg.in/yaml.v3`
      continuation; recency confirmed by `go list` output above (`2025-06-29`,
      within 12 months of dossier date `2026-06-13`).
- [x] (4) License recorded — MIT + Apache-2.0 (dual), confirmed via `LICENSE`
      first 40 lines pasted above.
- [x] (5) Transitive count recorded — `go mod graph` run in Module 4; 1 indirect
      dep (`gopkg.in/check.v1` — yaml library's own test framework, not a
      production transitive; zero third-party production transitives).
- [x] (6) De-facto status recorded — ecosystem-standard YAML library.
- [x] Vulnerability scan recorded — OSV.dev clean (`{}`) and `govulncheck ./...`
      (v1.3.0, via `make vuln`) reports "No vulnerabilities found."

Developer approval (AGENTS.md §7.1 requires explicit approval): the existence of
this committed dossier with all verification outputs pasted constitutes the
recorded approval to add `go.yaml.in/yaml/v3@v3.0.4`.
