# Database architecture and migrations

## Choose the engine

SQLite is the default, embedded in the Go binary (no C compiler or database service):

```yaml
storage:
  db_path: ./data/xchats.db
  wa_device_db_path: ./data/whatsmeow.db
```

For PostgreSQL 16 or newer, set `DATABASE_URL` or `storage.database_url`:

```yaml
storage:
  database_url: postgres://xchats:password@localhost:5432/xchats?sslmode=disable
```

Use deployment secrets and appropriate TLS settings in production. `DATABASE_URL`
takes precedence over `db_path`. A `postgres://` or `postgresql://` URL passed as
`db_path` also works for repository callers; prefer `database_url` in configuration.
The database must already exist. Install PostgreSQL's `citext` extension package;
the migration role needs schema DDL and permission to install this trusted extension
(or ask the database administrator to install it first).

Every repository opens through `internal/dbx`. SQLite handles share one connection,
WAL, enabled foreign keys, an immediate write transaction, and a process file lock.
PostgreSQL handles share a bounded pool, with UTC sessions and native pgx error
classification. Existing multi-statement read/modify/write operations retain their
serialization guarantee through a transaction advisory lock per database/schema.
Standalone queries can use the pool concurrently; import workers claim distinct rows
using `FOR UPDATE SKIP LOCKED`. Finer-grained transaction locks can replace the
schema-wide lock after the affected invariants receive concurrency tests.

WhatsApp device persistence follows the same engine toggle. With SQLite it uses a
separate file. With PostgreSQL the provider library manages its `whatsmeow_*` tables
in the selected database. Its vendor-owned schema and version table remain under
that library's own upgrade mechanism, outside the application's baseline.

## Schema and runner

```text
backend/
  migrations/
    migrations.go                              # embeds the *.sql files (migrations.FS)
    create.go                                  # UTC-timestamp migration generator
    schema_contract.json                       # shared relation/constraint test contract
    20261006000001_identity_access.sql         # 1/5 organizations, users, sessions, MCP OAuth, default admin
    20261006000002_channels_inbox.sql          # 2/5 wa_*, tg_*, channel_*, Meta bookkeeping, inbox views
    20261006000003_ai_knowledge_base.sql       # 3/5 assistant, knowledge tables, drafts, KB-gap telemetry, KB chat
    20261006000004_crm.sql                     # 4/5 customers, identities, statuses, tags, notes, follow-ups
    20261006000005_campaigns_automation.sql    # 5/5 campaigns, templates, send limits, channel automation
  internal/dbx/                                # engine access, dialect helpers, migration runner and renderer
  cmd/migration/                               # developer file-generation command
```

There is **one file per migration, shared by both engines**. Today's five files hold the
whole schema: 70 application tables, four inbox views, their constraints and indexes, and
the required organization/admin/CRM seed rows. Files 2-5 depend only on file 1. Each
file is plain SQL plus a few dialect macros (next section) that the runner renders for
the open engine before executing it. The shared contract and integration tests guard
divergence between the engines; there is no sequential migration history to replay. The
embedded files ship in the binary.

`store.New` and the other repository constructors select the dialect and migrate
before returning. The CLI can migrate without starting a server. Each file and its
history insert commit in one transaction. On failure both roll back. The runner
serializes migration checks and execution across callers (and PostgreSQL processes),
records `identifier`, SHA-256 `checksum`, and UTC `applied_at` in `schema_migrations`,
and rejects changed applied files. It does not maintain old migration tables.

The complete `YYYYMMDDHHMMSS_description` is the identifier, **not just the timestamp**.
Two descriptive names created in the same second are distinct. The generator refuses
to overwrite an existing name. Files are sorted for deterministic initial application;
every unapplied identifier is eligible, including one older than the latest applied
file. An out-of-order migration must still satisfy its real schema dependencies.

### Dialect macros

The runner renders every file with Go's `text/template` for the open engine, before
anything is applied: a file that does not render fails the run without applying any
earlier file. The checksum is the SHA-256 of the **unrendered** file, so it means the
same on every engine.

| Macro | SQLite | PostgreSQL |
| --- | --- | --- |
| `{{uuid}}` | random UUIDv4 expression | `(gen_random_uuid()::text)` |
| `{{now}}` | `(strftime('%Y-%m-%d %H:%M:%f','now'))` | `(xchats_now())` |
| `{{timestamp}}` | `TEXT` (UTC, milliseconds) | `TIMESTAMPTZ` |
| `{{json "col"}}` | `TEXT CHECK (col IS NULL OR json_valid(col))` | `JSONB` |
| `{{citext}}` | `TEXT COLLATE NOCASE` | `CITEXT` |
| `{{bytes}}` | `BLOB` | `BYTEA` |
| `{{if sqlite}}…{{end}}` | statements only SQLite runs | skipped |
| `{{if postgres}}…{{end}}` | skipped | statements only PostgreSQL runs |

`{{json "col"}}` takes the name of the column being declared (lower snake case), e.g.
`meta {{json "meta"}} NOT NULL DEFAULT '{}'`.

- Macros are **frozen once shipped**: an applied file is immutable, so changing what a
  macro expands to would silently diverge new databases from existing ones. Add a new
  macro instead (in `internal/dbx/render.go`, with its test).
- A file whose render has no statements (blank or only comments, e.g. everything sits in
  blocks for the other engine) is valid: it is recorded as applied and runs as a no-op.
- Write a literal double brace in SQL as `{{"{{"}}`. Do not put `{{` in comments.

## Type and query rules

| Concern | SQLite | PostgreSQL | In a migration |
| --- | --- | --- | --- |
| Application identifiers | TEXT UUIDs | TEXT UUIDs | `TEXT … DEFAULT {{uuid}}` |
| Timestamps | UTC ISO date/time TEXT, milliseconds (`2006-01-02 15:04:05.000`) | TIMESTAMPTZ | `{{timestamp}}`, `DEFAULT {{now}}` |
| Booleans | BOOLEAN, stored 0/1; use Go bool and SQL TRUE/FALSE | BOOLEAN | `BOOLEAN … DEFAULT FALSE` |
| Integers | INTEGER or BIGINT, both stored as 64-bit | INTEGER (32-bit) or BIGINT | `INTEGER`; `BIGINT` for ids and counters that can pass 2^31 |
| JSON and lists | TEXT with `json_valid` checks | JSONB | `{{json "col"}}` |
| Binary secrets | BLOB | BYTEA | `{{bytes}}` |
| Email uniqueness | TEXT COLLATE NOCASE | CITEXT | `{{citext}}` |

All current generated primary keys use UUIDs, avoiding sequence allocation differences.
For future integer keys pair SQLite `INTEGER PRIMARY KEY` with PostgreSQL
`BIGINT GENERATED BY DEFAULT AS IDENTITY`; retrieve keys with `RETURNING`, never
`LastInsertId`. UUIDs generated in application code use `uuid.New()`.

- Bind values with `$1`, `$2`, etc. Never interpolate user values or identifiers.
- Pass `time.Time` / `*time.Time`; let dbx encode SQLite timestamps and preserve
  PostgreSQL's native type. Do interval arithmetic in Go and bind the cutoff.
- Use `xchats_now()` for the shared SQL clock (SQLite driver function / PostgreSQL
  baseline function). Engine-specific clock expressions belong only in schema files.
- Use native `TRUE`/`FALSE` and scan into Go `bool`; do not scan booleans into integers.
- Use `RETURNING` and `ON CONFLICT`. Qualify existing columns inside an upsert,
  e.g. `base_version = kbd_draft.base_version + 1`.
- JSON scalar paths use shared `->` / `->>` operators. Use dbx `JSONValues` for
  array membership and `JSONMergePatch` for RFC 7396 recursive merge/delete semantics.
  These helpers take trusted SQL expressions/placeholders, not user values.
- Do not compare serialized JSON bytes: JSONB can reorder keys and whitespace.
- Use `unicode_lower` for Unicode case-insensitive search. PostgreSQL should use a
  UTF-8 locale with Unicode case rules, e.g. `C.utf8`. Email equality retains each
  engine's collation behavior; normalize email addresses before relying on non-ASCII
  equivalence across engines.
- For optional parameters, put a typed expression before an `IS NULL` test, e.g.
  `message_ts < $2 OR $2 IS NULL`, so PostgreSQL can infer the parameter's type.
- Explicitly type NULL branches in PostgreSQL UNION views when required.
- Keep PRAGMAs, SQLite backup commands, and PostgreSQL administration SQL in their
  engine-specific boundary. Built-in file backup/restore/check are SQLite tools;
  use `pg_dump`, `pg_restore`, and PostgreSQL administration tools for PostgreSQL.

## Developer workflow

From the repository root:

```sh
make migration-new NAME=contacts_lookup_index
```

Or from `backend/`:

```sh
go run ./cmd/migration contacts_lookup_index
go run ./cmd/xchats -config ../config.yaml migrate
DATABASE_URL='postgres://xchats:password@localhost:5432/xchats?sslmode=disable' \
  go run ./cmd/xchats -config ../config.yaml migrate
```

This creates **one** file, `backend/migrations/<UTC timestamp>_contacts_lookup_index.sql`,
for both engines. Write it as plain SQL plus the macros above, and update the shared
contract when relations change.
Use `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, and seed inserts
with `ON CONFLICT DO NOTHING`. Never overwrite an operator's data in baseline seeds.
PostgreSQL functions use `CREATE OR REPLACE`; views are dropped and recreated
(`DROP VIEW IF EXISTS` + `CREATE VIEW`) because SQLite has no `CREATE OR REPLACE VIEW`.

For constraints and column changes, put each engine's DDL in its own
`{{if postgres}}` / `{{if sqlite}}` block of the same file. PostgreSQL supports
conditional blocks and catalog checks. SQLite has no `ADD COLUMN IF NOT EXISTS`: design a
repeatable table rebuild/copy with explicit columns and preserved foreign keys, or use an
idempotent replacement table. Do not hide arbitrary DDL errors. Run each new file twice on
both engines, including a database with representative rows. Keep transaction
control out of scripts: the runner owns BEGIN/COMMIT/ROLLBACK. Operations that cannot
run inside a transaction (e.g. `CREATE INDEX CONCURRENTLY`) need a separate, explicit
operational procedure and do not belong in these files.

Applied files are immutable. During development only, explicitly replay a file or
all files after an intentional edit:

```sh
cd backend
go run ./cmd/xchats -config ../config.yaml migrate -force 20261006000003_ai_knowledge_base
go run ./cmd/xchats -config ../config.yaml migrate -force all
```

Replay records the new checksum. It does not infer whether arbitrary SQL is safe.
The CLI refuses forced replay when `environment: production` is configured.

**Reset development:** stop the app; remove the disposable SQLite database and its
`-wal`/`-shm` files, or drop and recreate the disposable PostgreSQL database; then run
`migrate`. Do not delete migration history while retaining a mismatched schema.
Databases created by the earlier single-baseline build (`20260929000000_baseline`) upgrade
in place: every statement is idempotent, so the five files apply on top and leave existing
data untouched. Databases from before that rewrite (numbered `0001_…` migrations) are
intentionally unsupported: recreate them.

## Verification

```sh
make test-backend
# Use a disposable database; each test creates and drops an isolated schema.
TEST_DATABASE_URL='postgres://xchats:password@localhost:5432/xchats_test?sslmode=disable' \
  make test-postgres
```

The PostgreSQL test role needs `CREATE SCHEMA`, `DROP SCHEMA`, and `citext` installed
in `public` (or permission to install it). `make test-postgres` runs the whole backend
suite, every package, on PostgreSQL, including the seeded composition-root test in
`cmd/xchats`; SQLite tests need no service. The default CI job runs the SQLite suite
only, so run `make test-postgres` before changing persistence code. Tests cover schema
parity, constraints/cascades, replay, checksum rejection, late arrivals, transaction
rollback, concurrent runners, template rendering, and application persistence behavior.

For a manual end-to-end check on either engine, seed the demo data and serve it in
mock-externals mode (no real credentials needed):

```sh
cd backend
export MOCK_EXTERNALS=true XCHATS_ALLOW_FILE_CREDENTIALS=1 ENVIRONMENT=development
go run ./cmd/xchats migrate && go run ./cmd/xchats seed && go run ./cmd/xchats serve
```

Set `DATABASE_URL` first to run the same flow on PostgreSQL.
