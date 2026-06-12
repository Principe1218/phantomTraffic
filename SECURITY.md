# Security Policy

PhantomTraffic is a tool for **authorized** network testing. We take the security of the
tool itself seriously and appreciate responsible disclosure.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues, discussions,
or pull requests.**

Instead, report them privately through
**[GitHub Security Advisories](https://github.com/Principe1218/phantomTraffic/security/advisories/new)**
("Report a vulnerability"). This keeps the report private until a fix is available.

> **Maintainer note:** enable **Settings → Code security and analysis → Private
> vulnerability reporting** for the link above to work. If you prefer email-based reports,
> add a dedicated security contact address here.

When reporting, please include:

- A description of the vulnerability and its impact.
- Steps to reproduce or a proof of concept.
- The affected version / commit.
- Any suggested remediation, if you have one.

**Please do not include real internal hostnames, IP addresses, credentials, or other
sensitive data** in your report — sanitize examples first.

## What to expect

- Acknowledgment of your report as soon as practical.
- An assessment and, if confirmed, a fix tracked privately until release.
- Credit for the disclosure if you would like it.

## Supported versions

PhantomTraffic is in early development. Until a tagged release exists, security fixes are
applied to the latest `main`.

## Scope

This policy covers vulnerabilities in PhantomTraffic itself (for example: credential
handling, unsafe defaults, code execution). It does **not** cover misuse of the tool
against systems you are not authorized to test — see the
[Authorized Use](README.md#-authorized-use-only) notice.
