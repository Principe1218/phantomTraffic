# PhantomTraffic

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Principe1218_phantomTraffic&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Principe1218_phantomTraffic) [![Bugs](https://sonarcloud.io/api/project_badges/measure?project=Principe1218_phantomTraffic&metric=bugs)](https://sonarcloud.io/summary/new_code?id=Principe1218_phantomTraffic)

**Realistic network traffic generator for firewall ACL validation.**

PhantomTraffic generates convincing, human-like network traffic across multiple
protocols so security and DevOps teams can verify that firewall rules and ACLs behave
as intended on restricted networks.

> 🚧 **Status: early development.** The design is set; the tool is not yet released.
> There are no pre-built binaries yet. APIs, config formats, and features below are
> subject to change.

---

## ⚠️ Authorized Use Only

**PhantomTraffic generates real network traffic against the targets you configure.**
Only run it against systems you **own** or have **explicit, written permission** to test.

Unauthorized use against networks or systems you do not control may violate computer-crime
laws (e.g., the U.S. Computer Fraud and Abuse Act, the UK Computer Misuse Act, and
equivalents elsewhere) and may breach the terms of service of third-party providers.
**You are solely responsible for how you use this tool.** See the full
[Disclaimer](#disclaimer) below.

---

## What it does

- Simulates genuine human usage patterns to validate ACLs without tripping anomaly detection.
- Tests whether firewall rules block what they should and allow what they should.
- Runs client-only — no server or central coordinator. One binary, run anywhere.
- Drives traffic from an interactive desktop UI **or** a headless CLI (feature-equivalent).

## Features

**Protocols:** HTTP(S), SSH / SFTP / SCP, DNS, streaming. Extensible via a common
handler interface.

**Realism (planned, MVP):**

- Time-of-day traffic profiles (weekday/weekend curves)
- Randomized think-time / jitter with configurable distributions
- Persona profiles (developer / office worker / admin)
- User-Agent and header rotation
- Bursty, clustered activity rather than uniform load

**Orchestration (planned, MVP):**

- Multiple targets per protocol with `random` or `sequential` rotation and configurable
  rotation intervals
- Built-in scheduling (e.g., Mon–Fri 8am–6pm) — no external cron
- Weighted, concurrent scenario mixes
- Ramp-up / ramp-down

**Safety & control (planned, MVP):**

- Rate limiting and safety caps to avoid accidentally overloading infrastructure
- Graceful pause / resume / stop
- Live dashboard (throughput, active connections, per-target success/fail, latency)

## How it works

Scenarios are defined in the UI or as YAML, then executed by a shared traffic engine:

```bash
phantom run scenario.yaml         # Run a saved scenario
phantom generate --config c.yaml  # One-off generation
phantom validate scenario.yaml    # Validate without sending traffic
phantom list-protocols            # Show available protocols
```

*(CLI shown for reference — not yet available in a released build.)*

## Built with

Go · [Wails](https://wails.io) · React + TypeScript

---

## Disclaimer

THIS SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, express or implied,
as set out in the [Apache License 2.0](LICENSE).

By using PhantomTraffic you acknowledge and agree that:

- **You are responsible for obtaining authorization.** Only generate traffic against
  systems you own or are explicitly permitted in writing to test.
- **This tool sends real traffic** and, if misconfigured, can degrade, disrupt, or
  effectively deny service to networks and hosts. You assume all operational risk.
- **You must comply with all applicable laws,** organizational policies, and the terms
  of service of any third-party services you direct traffic toward.
- **The authors and contributors accept no liability** for any damage, loss, legal
  consequence, or service disruption arising from use or misuse of this software, to the
  fullest extent permitted by law.

If you do not agree with these terms, do not use this software.

## License

Licensed under the [Apache License 2.0](LICENSE).

## Contributing

Contributions are welcome. By submitting a contribution, you agree it is licensed under
the Apache License 2.0 (see the license for contribution terms).
