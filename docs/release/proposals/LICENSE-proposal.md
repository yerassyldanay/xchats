# License proposal

**Status: proposal only.** This repository has no `LICENSE` file today,
which by default means "all rights reserved" — nobody but the copyright
holder may legally copy, modify, or redistribute this code, even though it's
hosted on a public-looking platform. Choosing a license is a legal and
business decision only the repository owner can make; this document lays
out the tradeoff space and a recommendation, not a decision.

## Recommendation: AGPL-3.0-only

The [GNU Affero General Public License v3.0](https://www.gnu.org/licenses/agpl-3.0.html).

### Why this one, for this project specifically

xchats is a **network service**, not a library or a CLI tool someone links
into their own program. A plain GPL's copyleft only triggers on
*distributing* the software — a company can run a modified GPL'd server
in-house or as a hosted SaaS forever and never trigger the obligation to
share their changes, because they never "distributed" anything. AGPL closes
exactly that gap: §13 extends the share-alike obligation to running a
modified version as a network service that users interact with remotely.

For a self-hosted team-inbox product like this one, that's the difference
between:

- **MIT/Apache-2.0** — maximum adoption, zero obligation on anyone who takes
  this code, modifies it, and re-sells it as a closed competing hosted
  product without contributing anything back.
- **AGPL-3.0** — anyone who runs a modified version as a service their own
  users interact with must make their modified source available to those
  users. Individuals and companies self-hosting xchats internally for their
  own team's use are unaffected either way — the obligation only activates
  when you offer the (modified) software to others over a network.

This mirrors the license choice made by comparable self-hosted communication
tools (e.g. Rocket.Chat, and formerly Mattermost's server component) for
exactly this reason.

### What AGPL means in practice for this repo

- Anyone can clone, self-host, and modify xchats for their own use, freely,
  with no obligation to share anything.
- A company that takes xchats, adds features, and offers it as a hosted
  product to third parties must offer those third-party users the modified
  source under AGPL too.
- It does **not** restrict what data flows through a deployment, what
  integrations you connect, or how you use it internally — only what
  happens if you redistribute or offer a modified version to others as a
  service.
- It's compatible with plain GPLv3 code as a dependency; it is **not**
  compatible with including this code inside a proprietary product without
  triggering the above.

### Alternatives considered

| License        | Tradeoff |
|-----------------|----------|
| **MIT / Apache-2.0** | Simplest, most adoption-friendly, but a hosted-SaaS fork of xchats could compete commercially while contributing nothing back — no obligation at all for network use. Apache-2.0 additionally grants an explicit patent license, which MIT doesn't; worth a look if patent concerns matter to the owner. |
| **GPL-3.0** | Closes the "modify and redistribute a binary" gap MIT/Apache leave open, but NOT the SaaS gap — running a modified version as a hosted service never counts as "distribution," so a hosted competitor still owes nothing back. |
| **BSL (Business Source License)** | Used by some infra/database companies (CockroachDB, Sentry) to delay open-source obligations for N years, converting to Apache/MIT later, while blocking commercial hosted competitors immediately. More restrictive than AGPL in the short term (usage restrictions beyond copyleft); more legal complexity to administer (a specific "Additional Use Grant" and conversion date to define and track). Worth considering only if the owner specifically wants to monetize a hosted xchats offering exclusively for some period. |
| **Proprietary / no license (status quo)** | What exists today by default. Keeps all rights reserved but also means nobody outside the owner can legally contribute, fork for their own legitimate self-hosted use, or build on it at all — likely not the intent if this is meant to be a usable open self-hosted product. |

### If AGPL is adopted

1. Add a `LICENSE` file at the repo root with the full AGPL-3.0 text (from
   https://www.gnu.org/licenses/agpl-3.0.txt).
2. Add an `SPDX-License-Identifier: AGPL-3.0-only` header convention for new
   source files going forward (retroactively stamping every existing file
   is optional, not required — the root `LICENSE` file alone is legally
   sufficient).
3. Reconcile with every third-party dependency's own license — see
   [`THIRD_PARTY_NOTICES-proposal.md`](THIRD_PARTY_NOTICES-proposal.md);
   AGPL is compatible with the permissive (MIT/BSD/Apache-2.0) licenses this
   project's dependencies use today, so no conflict was found.
4. Update `README.md`'s footer / add a `LICENSE` badge.

This choice is reversible only in one direction: once external contributors
submit code under AGPL, relicensing to something more permissive later
generally needs either their consent or a Contributor License Agreement
obtained up front. If there's any chance of wanting a more permissive
license later (e.g. a future dual-licensing/commercial-license business
model), consider requiring a CLA from contributors from day one — see
[`CONTRIBUTING-proposal.md`](CONTRIBUTING-proposal.md).
