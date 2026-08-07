# Upgrade and rollback

## How migrations work

Every schema change ships as a numbered, embedded SQL file under
`backend/migrations/sqlite/` (`0001_core.up.sql`, `0002_channels.up.sql`,
...). They're compiled into the binary (`go:embed`) and applied
automatically: `internal/store.New` — called on every boot, `xchats serve`
included — applies every migration newer than the database's current
version before the app does anything else. There is no separate `migrate`
step to remember; `xchats migrate` exists for driving migrations without
also starting the server (a deploy step that wants schema-ready-but-not-yet-
serving-traffic).

**There are no down-migrations.** Every migration file is forward-only
(`*.up.sql`, no `*.down.sql` counterpart). This is a deliberate simplicity
choice, not a gap to fill later — it means rollback is never "run the schema
backward," it's always **restore the pre-upgrade backup** (see below).

## Upgrading

1. **Back up first.** Non-negotiable — see
   [`backup-restore.md`](backup-restore.md). A new version's migrations run
   automatically and irreversibly on next boot; the backup is what makes
   that safe to attempt.
2. **Read the changelog** for the version you're moving to (see
   `proposals/CHANGELOG-format-proposal.md`)
   for anything that needs action beyond "start the new version" — a new
   required Setting, a changed default, a deprecation.
3. **Deploy the new version.**
   - Docker: pull/rebuild the new image, `make down` then `make up` (or
     `docker compose up -d --build` directly — see
     [`docker.md`](docker.md)). Volumes (and everything on them) survive a
     recreate.
   - Source/binary install: replace the binary, restart the process running
     `xchats serve`.
4. **Watch it boot.** Migrations apply automatically at this point — check
   logs for a clean startup. `xchats check` afterward confirms the database
   is structurally healthy.
5. **Verify.** Log in, confirm chats/messages/KB content look right, confirm
   configured integrations (Settings → Integrations) still show as
   configured. The Settings page's update notice (Settings → Data & Backup)
   should now report you're on the latest version.

## Rolling back

Since there's no down-migration path, rollback is: **stop the new version,
restore the backup taken before upgrading, start the old version.**

1. Stop the app (`make down`, or kill the process).
2. `xchats restore /path/to/pre-upgrade-backup.db /path/to/xchats.db` — see
   [`backup-restore.md`](backup-restore.md) for the full mechanics
   (destination must not be open; the backup is integrity-checked before
   anything is touched).
3. Redeploy the **previous** version's binary/image.
4. Start it back up.

This loses anything written between the backup and the rollback (new
messages, KB edits, new credentials saved) — there is no partial/selective
rollback. If that gap matters, prefer fixing forward (a patch release)
over rolling back once real traffic has flowed through the new version.

## Version identification

`internal/version.Version` (currently `0.1.0` until a real release process
starts stamping it from a git tag — see
[`release-checklist.md`](release-checklist.md)) is the single source both
the running app's Settings page and the update checker
(`GET /settings/update-check`) read from. Check it with:

```bash
curl -s http://localhost:8080/xchats/api/v1/settings/update-check
```

## Data compatibility notes

- **WhatsApp session state** (`whatsmeow.db`) is independent of the main
  `xchats.db` schema — an xchats upgrade never re-pairs WhatsApp numbers.
- **Credentials** are untouched by any upgrade or migration — they live in a
  completely separate store (see [`credentials.md`](credentials.md)) that no
  migration file ever touches.
- A version's migrations only ever move a database **forward**. Do not
  attempt to run a newer version's binary against a database, then
  downgrade the binary again without restoring a backup — the schema will
  be ahead of what the older binary expects.
