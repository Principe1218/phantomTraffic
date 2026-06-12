# Contributing to PhantomTraffic

Thanks for your interest in contributing! This document explains how to propose changes.

PhantomTraffic is a tool for **authorized** network testing. Please read the
[Authorized Use](README.md#-authorized-use-only) notice and the guidelines below before
contributing.

## Ground rules

- **All changes go through pull requests.** There are no direct pushes to `main`. Every
  change — including from the maintainer — is made via a PR that is reviewed before
  merging.
- **The maintainer reviews and merges.** Open a PR; it will be reviewed and merged once
  it meets the bar below. Not every PR will be accepted.
- **Contributions are licensed under Apache 2.0.** By opening a PR you agree your
  contribution is licensed under the [Apache License 2.0](LICENSE), per Section 5 of that
  license.
- **Scope guardrail.** PhantomTraffic exists for authorized firewall/ACL validation.
  Contributions whose primary purpose is to enable unauthorized access, evade detection
  for malicious ends, or otherwise facilitate abuse will be declined.

## How to contribute

1. **Open an issue first** for anything non-trivial — bugs, features, or design changes.
   This avoids wasted work. Use the issue templates.
2. **Fork** the repository and create a branch from `main`.
3. **Make your change**, with tests where applicable.
4. **Open a pull request** against `main`. Fill out the PR template, link the related
   issue, and describe what changed and why.
5. **Respond to review.** The maintainer will review; address feedback by pushing updates
   to your branch.

## Pull request expectations

- Focused scope — one logical change per PR.
- Tests pass and new behavior is covered.
- Clear description and a linked issue.
- Docs/README updated if behavior or usage changed.
- No secrets, real internal hostnames/IPs, or credentials in code, tests, or examples.

## Development setup

> 🚧 The build is still being stood up; setup details will firm up as the project matures.

PhantomTraffic is built with Go, [Wails](https://wails.io), and React + TypeScript. You
will need Go, Node.js, and the Wails CLI installed. Build/run instructions will be added
here once the initial scaffold lands.

## Reporting security issues

**Do not report security vulnerabilities through public issues.** See
[SECURITY.md](SECURITY.md) for the private disclosure process.
