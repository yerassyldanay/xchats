# Changelog format — proposal

**Status: proposal.** No `CHANGELOG.md` exists today. This proposes adopting
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format, tied to
[Semantic Versioning](https://semver.org/) — matching the versioning
approach already recommended in
[`../release-checklist.md`](../release-checklist.md) for
`internal/version.Version`.

## Why Keep a Changelog

- Human-readable and diffable — a plain Markdown file, reviewable in a
  normal PR like any other doc change, not a generated artifact only a tool
  can produce sensibly.
- Groups entries by *kind of change* (Added/Changed/Fixed/...), which is
  what a reader deciding whether to upgrade actually wants to scan for —
  not a flat commit list, which mixes internal refactors with things that
  affect an operator.
- Pairs naturally with [`CONTRIBUTING-proposal.md`](CONTRIBUTING-proposal.md)'s
  observed `type: summary` commit convention — a release's changelog entries
  can largely be assembled from `feat`/`fix`/`refactor` commits since the
  last tag, filtered for what's actually operator-visible.

## Format

```markdown
# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Settings UI: one-click backup download, self-healing provider status,
  update notifier.

### Changed
- Telegram now supports long-polling as well as webhook delivery,
  auto-selected based on whether a public base URL is configured.

### Fixed
- ...

### Security
- ...

## [0.1.0] - 2026-08-05

### Added
- Initial WhatsApp-first team inbox with AI-assisted draft/approve/send.
```

## Sections, in the order they appear when used

| Section      | What goes there |
|--------------|-------------------|
| `Added`      | New features. |
| `Changed`    | Changes to existing behavior. |
| `Deprecated` | Features slated for removal — say what to use instead. |
| `Removed`    | Features actually removed this release. |
| `Fixed`      | Bug fixes. |
| `Security`   | Vulnerability fixes — cross-reference
                 [`SECURITY-proposal.md`](SECURITY-proposal.md)'s disclosure
                 process; never include exploit detail here beyond what's
                 already public. |

Omit empty sections for a given release rather than listing them with
nothing under them.

## Workflow

1. **`[Unreleased]`** at the top accumulates entries as PRs land — each PR
   that changes operator-visible behavior adds its own entry under the
   right section, in the same PR (not reconstructed later from git log,
   which loses the "why this matters to an upgrader" framing a human writes
   better than a script does).
2. **At release time** (see
   [`../release-checklist.md`](../release-checklist.md)), rename
   `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD` and start a fresh empty
   `[Unreleased]` above it.
3. **Entries are for operators, not for other contributors** — "fixed a
   typo in a comment" doesn't belong here even though it's a real commit;
   "fixed the Telegram webhook secret not being verified on inbound
   requests" does. If a change doesn't affect anyone running xchats, it
   doesn't need a changelog entry, only a commit message.

## What NOT to do

- Don't auto-generate this file wholesale from `git log` — commit messages
  are written for reviewers of that specific diff, not for someone deciding
  whether to upgrade; the two audiences want different framing, and
  collapsing them produces a changelog nobody finds useful.
- Don't let entries pile up unwritten until release day — write them in the
  PR that makes the change, while the context for *why it matters to an
  operator* is still fresh.
