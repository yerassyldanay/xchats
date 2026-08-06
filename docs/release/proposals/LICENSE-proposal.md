# License proposal

**Status: adopted.** See the root [`LICENSE`](../../../LICENSE) file
(AGPL-3.0-only). This document records the rationale. It should still be
read alongside the correction below — the dependency-review finding
narrowed the choice before the network-service reasoning ever came into
play, and a final sign-off from counsel is worth getting before treating
this as closed.

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
| **MIT / Apache-2.0** | **Not actually available for the distributed binary/image** — see the dependency-review correction below; `go.mau.fi/libsignal` (GPL-3.0) is statically linked whenever WhatsApp connectivity ships. Listed here for completeness, not as a live option. |
| **GPL-3.0** | Legally workable (compatible with the linked libsignal dependency) but doesn't close the SaaS gap — running a modified version as a hosted service never counts as "distribution," so a hosted competitor still owes nothing back. This is the actual runner-up to AGPL-3.0, not MIT/Apache-2.0. |
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
   [`../../../THIRD_PARTY_NOTICES.md`](../../../THIRD_PARTY_NOTICES.md).
   **Correction to this proposal's original text:** an earlier pass of this
   document, and of `THIRD_PARTY_NOTICES-proposal.md`, claimed every
   dependency was permissively licensed and that "no conflict was found."
   That was wrong — `go.mau.fi/libsignal` (pulled in transitively through
   `go.mau.fi/whatsmeow`) is **GPL-3.0**, and dozens of its packages are
   statically linked into the `xchats` binary. That dependency is not
   optional if WhatsApp connectivity ships, which rules out MIT/Apache-2.0
   for the distributed binary or Docker image — not because AGPL was
   preferred over them in the abstract, but because they were never legally
   available once libsignal is in the build graph.
   GPL-3.0 and AGPL-3.0 are explicitly compatible with each other — see the
   FSF's own [GPL/AGPL compatibility FAQ
   entry](https://www.gnu.org/licenses/gpl-faq.en.html#AGPLGPL) — so this
   did not force AGPL specifically; GPL-3.0-only would also have been a
   legally workable choice. **AGPL-3.0 is selected as project policy after
   this dependency review**, on the network-service reasoning above, not
   because it was the only option. Treat this as a considered choice rather
   than a compelled one, and get a licensing-competent lawyer to confirm
   before leaning on it in a dispute.
4. Update `README.md`'s footer / add a `LICENSE` badge.

This choice is reversible only in one direction: once external contributors
submit code under AGPL, relicensing to something more permissive later
generally needs either their consent or a Contributor License Agreement
obtained up front. If there's any chance of wanting a more permissive
license later (e.g. a future dual-licensing/commercial-license business
model), consider requiring a CLA from contributors from day one — see
[`CONTRIBUTING-proposal.md`](CONTRIBUTING-proposal.md).
