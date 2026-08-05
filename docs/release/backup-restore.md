# Backup and restore

Two independent ways to get a backup, covering two different needs, plus one
restore path shared by both.

## Path 1 — Settings UI one-click download

**Settings → Data & Backup → Download Backup.** Streams a zip built by
`internal/dbops.BackupZip` containing:

- `manifest.json` — when the backup was created, plus the result of running
  an integrity check (`PRAGMA integrity_check`) against the live database at
  backup time; an empty result means healthy.
- `xchats.db` — a full database snapshot, taken via SQLite's `VACUUM INTO`
  (see below — safe against a live, running database).
- `settings.json` — the current non-secret settings (LLM defaults, ngrok
  region/domain, onboarding flags), only included when non-empty.
- `blob/` — every file under the configured blob directory, preserving its
  subdirectory structure.

**What it deliberately never includes: the credential store.** No API keys,
no system secrets, nothing from `credentials.enc`/`credentials.key`. This is
a documented, tested guarantee
(`TestBackupZip_NeverIncludesCredentialFiles`), not an oversight — a backup
file is handled and shared more casually than a credential store should ever
be, so the two are kept strictly separate. Restoring a backup onto a
deployment that already has its own credentials configured leaves those
credentials untouched.

This is the right choice for: routine backups an admin wants to keep offsite,
before a risky change, or as part of a manual rotation — anything that
doesn't need shell/CLI access to the machine xchats runs on.

## Path 2 — CLI

For scripted/scheduled backups (cron, a CI job, a NAS's backup hook), from
wherever the `xchats` binary already runs:

```bash
xchats backup /path/to/xchats-backup.db
```

Also uses `VACUUM INTO` — safe to run against a live, running database
(reads through SQLite's own consistent-snapshot machinery, unlike a raw
filesystem copy of the `.db` file, which can capture a half-written page).
Refuses to overwrite an existing destination file, and the output is fully
self-contained (no separate `-wal`/`-shm` sidecar to lose track of).

```bash
xchats check
```

Runs the same integrity check the Settings UI backup's manifest records,
against the live database, and exits non-zero with every problem logged if
any are found. Useful as a periodic health check independent of taking a
backup at all.

Note: this path produces a **raw `.db` file only** — no blob files, no
settings, no manifest. For a complete disaster-recovery artifact, prefer the
Settings UI download, or pair `xchats backup` with your own copy of the blob
directory.

## Restoring

```bash
xchats restore /path/to/backup.db /path/to/xchats.db
```

`internal/dbops.Restore` is an **offline operation** — the database at the
destination path must not be open by any running `xchats serve` process.
Restore acquires the same exclusive lock a running server holds
(`internal/dbx`'s single-process model), so:

- Running it while the server is up fails fast with a clear "currently open
  by another process — stop it before restoring" error, rather than racing
  the live process or corrupting it.
- **Stop the server first** (`make down` for Docker; kill the process for a
  source/dev run).

Before touching the destination at all, Restore validates the backup file
itself — opens it read-only and runs the same integrity check `xchats check`
uses. A missing, non-database, or internally corrupt backup file is rejected
with nothing at the destination touched. The actual replace is atomic: the
backup is copied to a temp file beside the destination, then renamed over
it, so a crash or interruption mid-restore leaves the destination as either
the complete old file or untouched — never a half-written one. Any stale
`-wal`/`-shm` sidecars belonging to the *previous* database at that path are
cleaned up afterward, so the next boot never replays a stale WAL against the
freshly restored content.

Restoring a `.db` file downloaded from the Settings UI's zip: extract
`xchats.db` from the archive first, then run `xchats restore` against that
extracted file. Blob files and settings from the same zip need to be placed
back manually (unzip `blob/` over the configured blob directory; re-apply
`settings.json`'s values from the Settings UI, since it isn't restored
automatically).

## What restore does NOT touch

Credentials — deliberately, since backups never contain them (see above). A
restored deployment keeps whatever credentials were already configured on
it before the restore. If you're restoring onto a *fresh* environment with
no credentials configured yet, you'll need to reconfigure integrations from
Settings (or the first-run wizard) after restoring.

## Recommended practice

- Take a backup before every upgrade (see
  [`upgrade-rollback.md`](upgrade-rollback.md) — restoring a pre-upgrade
  backup is the rollback procedure, since there are no down-migrations).
- Verify backups periodically by actually restoring one somewhere disposable
  — an unverified backup is a hope, not a plan.
- Store the file-backed credential store's key (`credentials.key`, if you're
  using that backend) separately from routine backups — see
  [`credentials.md`](credentials.md).
