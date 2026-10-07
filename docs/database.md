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
The database must already exist. Nothing else is required of it: the schema needs no
extension, no custom function and no superuser. The role only has to create tables,
indexes and views in its schema. Create the database with a UTF-8 locale (for example
`C.UTF-8` or `en_US.UTF-8`): case-insensitive search relies on PostgreSQL's `lower()`,
which only folds non-ASCII text such as Cyrillic when the database's character type
is UTF-8 aware.

Every repository opens through `internal/dbx`. SQLite handles share one connection,
WAL, enabled foreign keys, an immediate write transaction, and a process file lock.
PostgreSQL handles share a bounded pool, with UTC sessions and native pgx error
classification. Existing multi-statement read/modify/write operations retain their
serialization guarantee through a transaction advisory lock per database/schema.
Standalone queries can use the pool concurrently. Work that several workers contend
for, such as the import queue's claim, runs in such a transaction (or is a
compare-and-swap `UPDATE`), so no engine-specific locking clause is needed. Finer-grained
transaction locks can replace the schema-wide lock after the affected invariants receive
concurrency tests.

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
  internal/dbx/                                # engine access (the only engine-aware code) and the migration runner
  cmd/migration/                               # developer file-generation command
```

There is **one file per migration, shared by both engines**. Today's five files hold the
whole schema: 70 application tables, four inbox views, their constraints and indexes, and
the required organization/admin/CRM seed rows. Files 2-5 depend only on file 1. Each
file is **plain SQL, executed verbatim**: the same bytes run on SQLite and PostgreSQL,
with no templates, per-engine variants, extensions or functions. The shared contract and
integration tests guard divergence between the engines; there is no sequential migration
history to replay. The embedded files ship in the binary.

`store.New` and the other repository constructors select the dialect and migrate
before returning. The CLI can migrate without starting a server. Each file and its
history insert commit in one transaction. On failure both roll back. The runner
serializes migration checks and execution across callers (and PostgreSQL processes),
records `identifier`, SHA-256 `checksum` of the file, and `applied_at` (UTC Unix
milliseconds) in `schema_migrations`, and rejects changed applied files. It does not maintain old migration tables.

The complete `YYYYMMDDHHMMSS_description` is the identifier, **not just the timestamp**.
Two descriptive names created in the same second are distinct. The generator refuses
to overwrite an existing name. Files are sorted for deterministic initial application;
every unapplied identifier is eligible, including one older than the latest applied
file. An out-of-order migration must still satisfy its real schema dependencies.

## Portable SQL rules

The application runs identical SQL on both engines. Everything an engine would otherwise
generate or interpret for us is done in Go, and the schema uses one small type vocabulary:

| Concern | Column | Who supplies the value |
| --- | --- | --- |
| Identifiers | `TEXT PRIMARY KEY NOT NULL`, **no default** | Go: `uuid.New()` on every INSERT |
| Timestamps | `BIGINT NOT NULL`, **no default**: UTC Unix milliseconds | Go: bind a `time.Time`, once per method (`now := time.Now()`); dbx stores it as milliseconds and scans it back as a UTC `time.Time` |
| JSON documents and lists | `TEXT NOT NULL DEFAULT '{}'` or `'[]'`: no JSONB, no `json_valid` check | Go: `encoding/json` on write and read |
| E-mail | `TEXT NOT NULL UNIQUE`: no CITEXT, no `COLLATE NOCASE` | Go: `normalizeEmail` (lower case, trimmed) on insert, update and lookup |
| Booleans | `BOOLEAN NOT NULL DEFAULT FALSE` / `TRUE` | Go `bool`; SQL `TRUE`/`FALSE` |
| Binary secrets | `BYTEA` (SQLite accepts the name and stores the bytes as a BLOB) | Go `[]byte` |
| Numbers | `INTEGER`, `BIGINT` (ids and counters that can pass 2^31), `NUMERIC`, `REAL` | Go |

Rules for application SQL and migrations:

- Bind every value with `$1`, `$2`, … Never interpolate user values or identifiers.
- **No SQL clock, no SQL id generator.** There is no `now()`, `xchats_now()`,
  `gen_random_uuid()` or column default for either; INSERTs bind `id`, `created_at` and
  `updated_at` explicitly, and an upsert takes `updated_at = EXCLUDED.updated_at`. Do
  interval arithmetic in Go and bind the cutoff. Timestamps have millisecond resolution:
  two rows written in one millisecond tie, so order by an explicit sequence when order
  matters (see `chat_messages.seq`).
- **No SQL JSON.** Do not use `->`, `->>`, `json_*`/`jsonb_*`, merge-patch helpers or
  JSON table functions. Read the column, edit the value in Go and write it back. When the
  write races with others, make it a compare-and-swap on the text that was read; the
  import queue (`internal/kbstore/import.go`, `mutateImportParams`) is the worked
  example. A value the database must *filter* on is mirrored into its own column
  (`kbd_materials.import_run_id`, `import_synthesis_status`).
- **Lists.** `x IN <list>` uses `dbx.InList`, which builds `($n, …)` after the
  arguments already bound; `dbx.QueryInChunks` runs a read once per `dbx.MaxInList`
  (500) values. An empty list is `(NULL)`, which matches nothing, so a caller must handle
  "everything outside an empty list" itself. Do not use `= ANY($1)` or array parameters.
- **Case-insensitive search** is `lower(col) LIKE $n` with a pattern lower-cased in Go.
  dbx replaces SQLite's ASCII-only `lower()` with a Unicode-aware one; PostgreSQL's own
  is already Unicode-aware on a UTF-8 database.
- **No row-locking clauses** (`FOR UPDATE`, `SKIP LOCKED`). Serialise read-modify-write
  with a dbx transaction (SQLite `BEGIN IMMEDIATE`, PostgreSQL advisory lock) or a
  compare-and-swap `UPDATE`.
- Use `RETURNING` and `ON CONFLICT`. Qualify existing columns inside an upsert, e.g.
  `base_version = kbd_draft.base_version + 1`.
- Give PostgreSQL a type for every placeholder. Resolve `COALESCE($2, $3)`-style choices
  in Go and bind one value; for an optional parameter put a typed expression before the
  `IS NULL` test, e.g. `message_ts < $2 OR $2 IS NULL`. Type NULL branches of UNION
  views, e.g. `CAST(NULL AS BIGINT)`.
- Keep PRAGMAs, SQLite file-backup commands and PostgreSQL administration SQL inside the
  engine-specific boundary (`internal/dbx`, `internal/dbops`). Built-in file
  backup/restore/check are SQLite tools; use `pg_dump`, `pg_restore` and PostgreSQL
  administration tools for PostgreSQL.

All current primary keys are UUID strings, avoiding sequence differences. For a future
integer key, pair SQLite `INTEGER PRIMARY KEY` with PostgreSQL
`BIGINT GENERATED BY DEFAULT AS IDENTITY` only inside an engine boundary; retrieve keys
with `RETURNING`, never `LastInsertId`.

### SQL portability linter

These rules are enforced, not only documented. `internal/dbtest/sql_linter_test.go` runs
inside `go test` (so inside CI's `go test -race` and `make test-postgres` too) and as
`make lint-sql`, which `make lint-backend` runs first. It needs no database and opens no
connection, so it behaves the same with `TEST_DATABASE_URL` set or unset, and it runs under
`CGO_ENABLED=0`.

It reads every migration (the bytes that ship) and every string in `internal/` and `cmd/`
that reads as SQL, whichever function receives it. Text Go only knows at run time (a
joined `WHERE`, a `dbx.InList`) is replaced by a neutral stand-in so the rest is still
checked; other literals in a file that holds SQL are checked for engine-specific tokens.
Statements are parsed in memory by PostgreSQL's own parser (libpg_query, through the
pure-Go `github.com/wasilibs/go-pgquery`, which returns the `pg_query_go` parse tree).
Only string literals are read, never comments.

| Rule | Fails on |
| --- | --- |
| E1 | engine-specific tokens: `JSONB`, `TIMESTAMPTZ`, `CITEXT`, `->`/`->>`, `xchats_now`, `{{…}}`, `FOR UPDATE`, `COLLATE`, `ILIKE`, `::`, `PRAGMA`, `CREATE FUNCTION`/`EXTENSION`, `json_*()` and other engine-only functions |
| S1 | text PostgreSQL's grammar rejects, including SQLite-only syntax such as `INSERT OR REPLACE`; a statement with placeholders must be exactly one statement |
| P1 | placeholders other than `$1, $2, …`, gaps in the numbering, any placeholder in a migration |
| T1 | column and `CAST` types outside `text`, `bigint`, `integer`, `smallint`, `boolean`, `bytea`, `numeric`, `real` (`BLOB`, `UUID`, `TIMESTAMP`, `VARCHAR`, …) |
| D1 | a column `DEFAULT` that is not a constant |
| M1 | statements other than `SELECT`/`INSERT`/`UPDATE`/`DELETE` and `CREATE`/`DROP` of tables, indexes and views; `ALTER TABLE` other than `ADD COLUMN` and `RENAME` (SQLite cannot alter a column or constraint in place: rebuild the table) |
| N1 | constructs only one engine has: `DISTINCT ON`, `= ANY(…)`, `ARRAY[…]`, `CURRENT_TIMESTAMP`, `ROLLUP`, `LATERAL`, identity columns, operators other than `= <> < <= > >= + - * / % \|\|` |
| F1 | functions other than `count`, `sum`, `min`, `max`, `lower` (one argument each), `TRIM(x)` and `LIKE … ESCAPE` |

Each finding names `file:line` and the rule. Fix it at the source. A construct that is
portable but not yet allowed goes into the allowlists at the top of the linter (and its
fixture into the self-tests) only after it has run on both engines. Code that is
engine-specific by design is exempt, with a written reason, in `sqlLintExempt`:
`internal/dbx` (the engine boundary), `internal/dbops` (SQLite file backup), and the
PostgreSQL schema harness, fault injection and catalog tests in `internal/dbtest`.

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
for both engines. Write it as plain SQL that obeys the rules above, and update the shared
contract when relations change.
Use `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, and seed inserts
with `ON CONFLICT DO NOTHING`; seed rows carry explicit ids and millisecond timestamps.
Never overwrite an operator's data in baseline seeds. Views are dropped and recreated
(`DROP VIEW IF EXISTS` + `CREATE VIEW`) because SQLite has no `CREATE OR REPLACE VIEW`.
Do not create functions, triggers, sequences or extensions.

Stick to DDL both engines run: `CREATE TABLE/INDEX/VIEW`, `ALTER TABLE … ADD COLUMN`,
`DROP`, `RENAME`. SQLite cannot `ALTER COLUMN`, add or drop a constraint, so such a change
is a repeatable table rebuild in plain SQL (create the replacement table, copy with
explicit columns, preserve foreign keys, drop, rename), which runs unchanged on
PostgreSQL. Do not hide arbitrary DDL errors. Run each new file twice on both engines,
including a database with representative rows. Keep transaction control out of scripts:
the runner owns BEGIN/COMMIT/ROLLBACK. Operations that cannot run inside a transaction
(e.g. `CREATE INDEX CONCURRENTLY`) need a separate, explicit operational procedure and do
not belong in these files.

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
The five files were rewritten in place when the schema became plain portable SQL
(`TEXT` ids, `BIGINT` millisecond timestamps, no extensions), and the project is
unreleased, so a development database created by any earlier build is not upgraded: it
fails with a checksum mismatch on the first changed file. Recreate it (drop and recreate
the PostgreSQL database, or delete the SQLite file) and run `migrate`.

## Verification

```sh
make lint-sql        # SQL portability linter: no database needed
make test-backend
# Use a disposable database; each test creates and drops an isolated schema.
TEST_DATABASE_URL='postgres://xchats:password@localhost:5432/xchats_test?sslmode=disable' \
  make test-postgres
```

The PostgreSQL test role needs only `CREATE SCHEMA` and `DROP SCHEMA` on the test
database: no extension and no superuser. `make test-postgres` runs the whole backend
suite, every package, on PostgreSQL, including the seeded composition-root test in
`cmd/xchats`; SQLite tests need no service. The default CI job runs the SQLite suite
only, so run `make test-postgres` before changing persistence code. Tests cover schema
parity (`migrations/schema_contract.json`: names, types, nullability, defaults, keys and
indexes, identical on both engines), constraints/cascades, replay, checksum rejection, late arrivals, transaction
rollback, concurrent runners, the import queue's concurrency, and application persistence
behavior. `TestSchemaHasNoServerSideObjects` asserts the migrated PostgreSQL schema holds
no functions, sequences, triggers or extension types.

For a manual end-to-end check on either engine, seed the demo data and serve it in
mock-externals mode (no real credentials needed):

```sh
cd backend
export MOCK_EXTERNALS=true XCHATS_ALLOW_FILE_CREDENTIALS=1 ENVIRONMENT=development
go run ./cmd/xchats migrate && go run ./cmd/xchats seed && go run ./cmd/xchats serve
```

Set `DATABASE_URL` first to run the same flow on PostgreSQL.
