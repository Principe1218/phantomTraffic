# AGENTS.md — Security Engineering Standards

> This file governs how Claude Code agents reason about, write, and review code in this repository.
> Every agent session **must** internalize these rules before producing any output.
> These standards reflect the security posture of a software engineer operating at CISO-level expectations,
> broadly aligned with NIST SP 800-53 controls.

---

## 0. Overarching Principle

**Security is not a feature — it is a baseline.**

When in doubt, choose the more restrictive, more auditable, and more explicit option.
Never trade correctness or security for brevity or convenience.

### Simplicity is a security property

Simple code is easier to read, easier to audit, easier to test, and harder to exploit.
Complexity is where bugs hide — and in security-sensitive code, bugs become vulnerabilities.

- **Prefer the shortest correct solution.** If two implementations are equally correct and safe, choose the shorter one.
- **Prefer clarity over cleverness.** Code that a reviewer can understand in 30 seconds is better than elegant code that takes 10 minutes to reason about.
- **Avoid over-engineering.** Do not introduce abstractions, layers, or generalization that the current requirements do not justify. You can always add complexity later; removing it is harder.
- **One thing per function.** Functions and methods should do one thing and do it clearly. Long functions that mix concerns are harder to audit for security correctness.
- **Avoid deep nesting.** Prefer early returns and guard clauses over deeply nested conditionals — control flow that is hard to follow is control flow that is easy to get wrong.
- **Do not rewrite working code to be more sophisticated.** If existing code is simple, correct, and secure, leave it alone. Refactor only when there is a clear, demonstrable reason.
- **Flag your own complexity.** If you find yourself writing something non-trivial, add a concise comment explaining *why* — not what the code does, but why it must be done this way.

> A security auditor's time is finite. Every unnecessary line of code is a line they have to read.

---

## 1. Proactive Security Posture

### 1.1 Enforce on task, flag everything else

- **On every task**: apply all applicable rules in this document to the code being written or modified.
- **Opportunistically**: if you observe a security issue *outside* the scope of the current task (in surrounding code, imports, configs, etc.), surface it as a clearly labeled `⚠ SECURITY WARNING` comment at the end of your response — do not silently ignore it, and do not block the task over it unless it is critical (e.g., a hardcoded secret, an actively exploitable vuln in a direct code path).
- Never suppress a security concern because the user did not ask about it.

### 1.2 Format for out-of-scope warnings

```markdown
⚠ SECURITY WARNING (out of scope — not blocking this task)
Location: <file>:<line or function>
Issue: <concise description>
Recommendation: <what should be done>
NIST 800-53 Reference: <control family, e.g., SC-28, IA-5>
```

---

## 2. Cryptography

### 2.1 Never invent cryptography

- **Do not** implement custom encryption, hashing, PRNG, MAC, KDF, or signature schemes — ever.
- Do not combine primitives manually (e.g., "encrypt-then-MAC by hand") unless you are wrapping a well-audited library that already does this correctly.
- If a cryptographic need arises that no existing library covers cleanly, **stop and ask the developer** before proceeding.

### 2.2 Approved algorithms (use these; no others without explicit approval)

| Purpose | Approved | Forbidden examples |
| --- | --- | --- |
| Symmetric encryption | AES-256-GCM, ChaCha20-Poly1305 | DES, 3DES, RC4, AES-ECB, AES-CBC without authenticated encryption |
| Asymmetric encryption | RSA-OAEP (≥2048-bit, prefer 4096), ECIES | RSA-PKCS1v1.5 padding, raw RSA |
| Signatures | Ed25519, ECDSA (P-256/P-384), RSA-PSS | RSA-PKCS1v1.5 signing, DSA |
| Hashing (integrity) | SHA-256, SHA-384, SHA-512, BLAKE2b/BLAKE3 | MD5, SHA-1 (for any security purpose) |
| Password hashing / KDF | Argon2id (preferred), bcrypt (cost ≥ 12), scrypt | PBKDF2 with low iterations, plain SHA for passwords, unsalted hashing |
| Key derivation (non-password) | HKDF-SHA256/512 | Direct key reuse, truncation |
| Random number generation | `crypto/rand` (Go), `node:crypto` `randomBytes` | `math/rand`, `Math.random()`, any non-CSPRNG |
| TLS | TLS 1.2 (limited cipher suites) or TLS 1.3 | TLS 1.0, TLS 1.1, SSL, `InsecureSkipVerify: true` |

### 2.3 Go-specific cryptography

Prefer the standard library (`crypto/*`) and `golang.org/x/crypto`. Do not import third-party crypto packages without explicit developer approval. Prefer `golang.org/x/crypto/chacha20poly1305` or `crypto/cipher` (AES-GCM) for symmetric encryption.

### 2.4 TypeScript/Node.js-specific cryptography

Use `node:crypto` (built-in). For higher-level needs, prefer `@noble/ciphers`, `@noble/hashes`, or `jose` (for JWT/JWE). Do not use `crypto-js` — it is not actively maintained and has known issues. Do not use `sjcl`, `forge`, or similar legacy libraries unless pre-existing and flagged for replacement.

---

## 3. Secrets and Credentials Management

### 3.1 Hardcoded secrets — full stop

- **Never** write a secret, API key, token, password, certificate private key, or credential of any kind into source code, test files, configuration files committed to version control, or log output.
- If you encounter a hardcoded secret in existing code:
  1. Do **not** propagate it (do not copy/paste it, refactor around it, or generate tests using it).
  2. Surface it immediately as a `⚠ SECURITY WARNING` (see §1.2) with severity **CRITICAL**.
  3. Ask the developer how they want to handle it before proceeding with the task.

### 3.2 Preferred secrets patterns

- **Always ask the developer** which secrets manager is in use before writing secrets-access code.
- Approved patterns (in order of preference):
  1. Cloud-native secrets managers: AWS Secrets Manager, GCP Secret Manager, Azure Key Vault
  2. HashiCorp Vault (with short-lived dynamic credentials preferred over static ones)
  3. Environment variables injected at runtime by a secrets manager (not `.env` files committed to git)
- **Never** suggest `.env` files as a long-term secrets solution — only as a local-dev convenience with an explicit note that they must be in `.gitignore` and never committed.

### 3.3 Secret rotation and least privilege

- Secrets should be scoped to the minimum required permissions (least privilege, NIST AC-6).
- Where the platform supports it, suggest or generate code that uses short-lived credentials / token rotation rather than long-lived static keys.
- When generating IAM policies, service account scopes, or API tokens, always draft the minimal permission set needed — never `*` wildcards unless the developer explicitly approves.

---

## 4. Authentication and Authorization

### 4.1 Authentication

- Use established identity/auth libraries — do not implement authentication flows from scratch.
  - Go: `golang.org/x/oauth2`, `coreos/go-oidc`, `zitadel/oidc`
  - Node.js/TypeScript: `passport`, `openid-client`, `jose`
- For JWT:
  - Always validate `alg`, `iss`, `aud`, `exp`, `nbf`.
  - Never accept `alg: none`.
  - Never use symmetric JWT signing (`HS256`) for tokens that cross a trust boundary (e.g., between services with different keys). Prefer `RS256`/`ES256`/`EdDSA`.
  - Use short expiry windows (`exp`); always issue refresh tokens separately.
- Passwords must be hashed with Argon2id or bcrypt — never stored in plaintext or with reversible encryption.

### 4.2 Authorization

- Enforce authorization checks **server-side**, at the earliest possible point in the request lifecycle. Never trust client-supplied role or permission claims without verification.
- Implement RBAC or ABAC via a dedicated policy layer — do not scatter `if user.role == "admin"` checks inline throughout business logic.
- Default to **deny** — explicitly grant access, never implicitly allow it.
- When writing middleware or guards, make the deny branch the default `else`, not a fallthrough.

### 4.3 Session management

- Session tokens must be cryptographically random (≥128 bits), stored server-side, and invalidated on logout and privilege change.
- Cookies: always set `HttpOnly`, `Secure`, `SameSite=Strict` (or `Lax` where strict breaks required flows — document why).

---

## 5. Network and API Security

### 5.1 TLS everywhere

- All outbound HTTP connections **must** use HTTPS/TLS. Never use plain HTTP for any production code path.
- In Go: never set `InsecureSkipVerify: true` in `tls.Config` — if certificate validation is a problem, fix the certificate, not the validator.
- In Node.js: never set `rejectUnauthorized: false` — same rule applies.
- Pin or validate CA bundles for high-trust internal service communication.

### 5.2 Input validation

- Validate and sanitize **all** external input (HTTP request bodies, query params, headers, gRPC fields, CLI args, file contents) at the entry point, before any processing.
- Use allowlists (permitted characters/values/ranges), not denylists.
- For Go: use `github.com/go-playground/validator` or struct-level validation. Never trust raw `interface{}` deserialized from user input.
- For TypeScript: use `zod` or `@sinclair/typebox` for runtime schema validation. Never trust `any`-typed input.

### 5.3 Output encoding

- Always context-encode output to prevent injection (HTML encoding for HTML, parameterized queries for SQL, shell escaping for CLI, etc.).
- Never construct SQL, shell commands, LDAP queries, or XML/HTML by string concatenation with user-controlled data.
- Use parameterized queries or prepared statements for **all** database access — no exceptions.

### 5.4 Rate limiting and abuse prevention

- All public-facing API endpoints should have rate limiting. Flag any endpoint added without it.
- Authentication endpoints (login, token refresh, password reset) must have stricter rate limits and account lockout/throttling logic.

### 5.5 Error handling

- Never leak stack traces, internal error messages, library versions, or infrastructure details in API responses.
- Log the full error server-side; return a generic, non-revealing error to the client.
- Use structured logging (no `fmt.Println` / `console.log` in production paths) with appropriate log levels.

---

## 6. Infrastructure and Cloud Security

- Never generate Terraform, Helm, Kubernetes YAML, or cloud configs with:
  - Public S3 buckets / GCS buckets without explicit developer confirmation and a documented reason.
  - Security groups / firewall rules opening `0.0.0.0/0` on non-HTTP/HTTPS ports.
  - Containers running as `root` — always specify a non-root `USER` in Dockerfiles.
  - Privileged containers (`privileged: true`) or `hostNetwork: true` without explicit approval.
  - Hardcoded credentials (see §3).
- Kubernetes: always set `readOnlyRootFilesystem: true`, drop all capabilities by default (`drop: ["ALL"]`), and add only what is required.
- IAM: default to least privilege (see §3.3). Flag `*` actions or `*` resources in IAM policies.

---

## 7. Dependency and Supply Chain Security

### 7.1 Zero new dependencies without explicit approval

- **Do not add any new dependency** (direct or indirect, production or dev) without asking the developer first.
- When proposing a new dependency, provide:
  1. Package name and version
  2. Purpose / why existing code or stdlib cannot solve this
  3. Maintenance status (last release, active maintainers, open CVEs)
  4. License
  5. Transitive dependency count (rough estimate)
  6. Whether it is the de-facto standard in the ecosystem

### 7.2 Dependency health criteria

Before recommending a library, it must meet **all** of:

- Actively maintained (release or meaningful commit in the last 12 months)
- No known critical/high unpatched CVEs (check [OSV.dev](https://osv.dev), [GitHub Advisory Database](https://github.com/advisories))
- Widely adopted or explicitly endorsed by the language/framework ecosystem
- Pinned to a specific version (no floating `latest` / `*` version ranges in production)

### 7.3 Lock files and integrity

- Always commit lock files (`go.sum`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`).
- For Go: verify `go.sum` entries are present for all dependencies.
- For Node.js: prefer `npm ci` over `npm install` in CI pipelines; use `--ignore-scripts` where possible.
- Flag any `postinstall` or lifecycle scripts in new dependencies as requiring manual review.

### 7.4 Vulnerability scanning

- Treat dependency vulnerability scanning as mandatory, not optional. Recommend:
  - Go: `govulncheck ./...`
  - Node.js: `npm audit` / `pnpm audit`
  - Cross-language / CI: `trivy`, `grype`, or `osv-scanner`

---

## 8. Secure Coding Patterns — Language-Specific

### 8.1 Go

- Always handle errors explicitly — never use `_` for error returns in security-sensitive code paths.
- Use `context.Context` with timeouts for all external calls (HTTP, DB, gRPC) — never unbounded waits.
- Avoid `unsafe` package unless absolutely necessary; flag any use for developer review.
- Use `sync.Mutex` or channels correctly — never share mutable state across goroutines without synchronization.
- Set `http.Client` timeouts explicitly (`Timeout`, `TLSHandshakeTimeout`, etc.) — the zero-value client has no timeout.
- Run `go vet`, `staticcheck`, and `gosec` as part of the standard build/CI pipeline.

### 8.2 TypeScript / Node.js

- Enable strict TypeScript compiler options: `strict: true`, `noImplicitAny: true`, `strictNullChecks: true`.
- Avoid `eval()`, `new Function()`, and dynamic `require()` with user-controlled input.
- Use `helmet` for Express/Fastify HTTP security headers; never serve APIs without it.
- Set explicit `Content-Security-Policy` headers for any service that renders HTML.
- Use `node:crypto` for all cryptographic operations — never `Math.random()` for security purposes.
- Run `eslint` with `eslint-plugin-security` and `@typescript-eslint` rules in CI.

---

## 9. Logging, Monitoring, and Auditability

- Log all authentication events (success and failure), authorization denials, and privilege escalations (NIST AU-2, AU-3).
- Never log PII, credentials, session tokens, or sensitive business data — log identifiers (user ID, request ID) instead.
- Ensure logs are structured (JSON), include a timestamp and a correlation/request ID, and go to a centralized, tamper-evident sink.
- Audit-critical operations (key rotation, permission changes, admin actions) must emit immutable audit log entries with actor, action, resource, and timestamp.

---

## 10. Security Review Triggers

Always pause and ask the developer before proceeding if you are about to:

- Add a new dependency (§7.1)
- Write any cryptographic code not using an approved library/primitive (§2)
- Generate code that opens a network port, firewall rule, or public endpoint
- Generate IAM policies, RBAC roles, or any permission assignments
- Encounter or need to reference a secret, credential, or key
- Generate code that processes, stores, or transmits PII or sensitive data
- Disable or weaken a security control (TLS verification, CSRF protection, auth checks)
- Write code that executes shell commands, subprocesses, or dynamic code evaluation

---

## 11. NIST SP 800-53 Quick Reference Mapping

| Section in this doc | NIST 800-53 Controls |
| --- | --- |
| Cryptography | SC-8, SC-12, SC-13, SC-28 |
| Secrets management | IA-5, SC-12, SC-28 |
| Authentication & authorization | IA-2, IA-8, AC-2, AC-3, AC-6 |
| Network & API security | SC-8, SC-23, SI-10, SI-15 |
| Infrastructure security | CM-6, CM-7, AC-6, SC-7 |
| Supply chain / dependencies | SA-12, SA-15, RA-5, SI-2 |
| Logging & audit | AU-2, AU-3, AU-9, AU-12 |
| Simplicity & auditability | SA-15, CM-6 (configuration/code clarity as a control) |

---

*Last updated: 2026-06-11 — Review and update this file when the tech stack, compliance requirements, or security posture changes.*
