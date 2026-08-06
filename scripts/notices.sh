#!/usr/bin/env bash
# scripts/notices.sh — regenerate THIRD_PARTY_LICENSES.txt from the actual
# dependency graph of what we ship:
#   Section 1 (Go):  every module statically linked into backend/cmd/xchats
#   Section 2 (npm): the production dependency tree bundled into the frontend
#
# Pinned tools — bump deliberately, together with a THIRD_PARTY_NOTICES.md
# table refresh:
#   go-licenses:      github.com/google/go-licenses/v2@v2.0.1 (via `go run`)
#   license-checker:  license-checker-rseidelsohn@4.4.2 (via `npx --yes`)
#     4.x pinned on purpose: 5.x requires Node >= 24; this repo pins Node 22
#     (.nvmrc, frontend/Dockerfile).
#
# Prereqs: Go toolchain per backend/go.mod, Node+npm, and a populated
# frontend/node_modules (run `npm ci` in frontend/ first). First run needs
# network for `go run @version` / `npx --yes` downloads; both cache after.
set -euo pipefail
export LC_ALL=C

GO_LICENSES='github.com/google/go-licenses/v2@v2.0.1'
LICENSE_CHECKER='license-checker-rseidelsohn@4.4.2'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/THIRD_PARTY_LICENSES.txt"
OVERRIDES_NPM="$ROOT/scripts/license-overrides/npm"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

SEP='================================================================================'

die() { echo "notices.sh: $*" >&2; exit 1; }

# Force module mode: a leftover backend/vendor/ (the release source-bundle
# job creates one; a local dry run may too) would flip go commands to
# -mod=vendor and break module-cache license paths.
export GOFLAGS=-mod=mod

# ---------------------------------------------------------------------------
# Section 1 — Go. `report --template` (not `save`): report tolerates
# unclassifiable licenses (emits LicenseName "Unknown" and keeps going),
# exposes the on-disk license path, and doesn't dump copyleft dep source —
# corresponding source ships via the release tarball instead.
# ---------------------------------------------------------------------------
printf '{{range .}}{{.Name}}\t{{.Version}}\t{{.LicenseName}}\t{{.LicenseURL}}\t{{.LicensePath}}\n{{end}}' \
  > "$TMP/go.tpl"

(cd "$ROOT/backend" && go run "$GO_LICENSES" report ./cmd/xchats --template="$TMP/go.tpl" 2>"$TMP/go-licenses.log") \
  | cat > "$TMP/go-manifest.raw.tsv" \
  || { cat "$TMP/go-licenses.log" >&2; die "go-licenses report failed"; }

# Drop this project's own packages (reported alongside real dependencies —
# obviously not third-party, and always "no license found" since LICENSE
# lives at the repo root, one level above backend/'s module root) and blank
# lines, then dedupe. Same (name, version, LicensePath) triple can appear
# more than once with a DIFFERENT LicenseName — go-licenses' text-similarity
# classifier occasionally scores one file as a close match for two license
# templates (observed: every go.opentelemetry.io/otel/* package's LICENSE
# classifies as both Apache-2.0 and BSD-3-Clause). That's the same file, so
# group and join the license names rather than printing identical text twice.
grep -v '^[[:space:]]*$' "$TMP/go-manifest.raw.tsv" \
  | grep -v '^github\.com/yerassyldanay/xchats/' \
  | sort -t "$(printf '\t')" -k1,1 -k2,2 -k5,5 -k3,3 \
  | awk -F'\t' -v OFS='\t' '
      function flush() { if (key != "") print name, version, licnames, licurl, licpath }
      { key2 = $1 FS $2 FS $5
        if (key2 != key) { flush(); key=key2; name=$1; version=$2; licnames=$3; licurl=$4; licpath=$5 }
        else if (index(licnames, $3) == 0) { licnames = licnames " / " $3 }
      }
      END { flush() }' \
  > "$TMP/go-manifest.sorted.tsv"

emit_entry() { # $1=name $2=version $3=license-id $4=source-url $5=license-file
  {
    printf '%s\n' "$SEP"
    if [ -n "$4" ] && [ "$4" != "Unknown" ]; then
      printf '%s@%s — %s — %s\n' "$1" "$2" "$3" "$4"
    else
      printf '%s@%s — %s\n' "$1" "$2" "$3"
    fi
    printf '%s\n\n' "$SEP"
    printf '%s\n\n\n' "$(cat "$5")"   # normalizes trailing newlines to exactly two blanks
  } >> "$SECTION"
}

emit_extra() { # $1=label $2=file — NOTICE / PATENTS riders for the entry above
  {
    printf -- '---- %s ----\n\n' "$1"
    printf '%s\n\n\n' "$(cat "$2")"
  } >> "$SECTION"
}

SECTION="$TMP/section-go.txt"; : > "$SECTION"
GO_COUNT=0
while IFS=$'\t' read -r name version licname licurl licpath; do
  if [ "$licname" = "Unknown" ] || [ -z "$licpath" ] || [ "$licpath" = "Unknown" ]; then
    # Classifier escape hatch. Known miss: inconshreveable/log15 (and its
    # /v3, /v3/ext sub-packages — all one module's LICENSE) ships a
    # short-form Apache-2.0 notice go-licenses' text classifier may not
    # match (see THIRD_PARTY_NOTICES.md). Anything unexpected fails the run.
    case "$name" in
      github.com/inconshreveable/log15*)
        licname='Apache-2.0' ;;
      *)
        cat "$TMP/go-licenses.log" >&2
        die "unclassified Go license for '$name' — verify by hand, then extend the override case in $0" ;;
    esac
    if [ -z "$licpath" ] || [ "$licpath" = "Unknown" ]; then
      # `$name` here is a PACKAGE import path, possibly a sub-package with no
      # go.mod of its own (e.g. log15/v3/ext) — resolve via the owning
      # module's directory (go list on the package), not `go list -m` (which
      # requires `$name` to itself be a module path).
      (cd "$ROOT/backend" && go mod download "$name" >/dev/null 2>&1) || true
      dir="$(cd "$ROOT/backend" && go list -f '{{.Module.Dir}}' "$name" 2>/dev/null || true)"
      [ -n "$dir" ] || die "cannot resolve module dir for '$name'"
      licpath="$(find "$dir" -maxdepth 1 \( -iname 'LICENSE*' -o -iname 'COPYING*' \) | sort | head -n1)"
      [ -n "$licpath" ] || die "no license file found in $dir for '$name'"
    fi
  fi
  [ -n "$version" ] || version='(unversioned)'
  emit_entry "$name" "$version" "$licname" "$licurl" "$licpath"
  # Apache-2.0 NOTICE files and golang.org/x PATENTS grants sit next to the
  # license file — required/valuable riders, so ship them too.
  while IFS= read -r extra; do
    [ -n "$extra" ] || continue
    emit_extra "$(basename "$extra")" "$extra"
  done < <(find "$(dirname "$licpath")" -maxdepth 1 \( -iname 'NOTICE*' -o -iname 'PATENTS*' \) | sort)
  GO_COUNT=$((GO_COUNT + 1))
done < "$TMP/go-manifest.sorted.tsv"
[ "$GO_COUNT" -ge 40 ] || die "only $GO_COUNT Go entries — report output looks broken (expect ~72)"

# ---------------------------------------------------------------------------
# Section 2 — npm production tree (direct + transitive: bundled code is what
# needs attribution, not just the direct deps).
# ---------------------------------------------------------------------------
[ -d "$ROOT/frontend/node_modules" ] || die "frontend/node_modules missing — run 'npm ci' in frontend/ first"

(cd "$ROOT/frontend" && npx --yes "$LICENSE_CHECKER" \
    --production --excludePrivatePackages --json) > "$TMP/npm-report.json"

NPM_REPORT="$TMP/npm-report.json" OVERRIDES="$OVERRIDES_NPM" node - > "$TMP/section-npm.txt" <<'NODE'
const fs = require('fs'), path = require('path');
const report = JSON.parse(fs.readFileSync(process.env.NPM_REPORT, 'utf8'));
const overrides = process.env.OVERRIDES;
const SEP = '='.repeat(80);
const looksLikeLicense = f => /licen[cs]e|copying|notice/i.test(path.basename(f));
let count = 0;
for (const key of Object.keys(report).sort()) {      // "name@version"; default sort = code-unit order, locale-independent
  const at = key.lastIndexOf('@');
  const name = key.slice(0, at), version = key.slice(at + 1);
  const e = report[key];
  const overrideFile = path.join(overrides, name.replace(/\//g, '__') + '.txt');
  let file, note = '';
  if (fs.existsSync(overrideFile)) {
    file = overrideFile;
  } else if (e.licenseFile) {
    file = e.licenseFile;
    if (!looksLikeLicense(e.licenseFile))            // license-checker fell back to a README
      note = ` (license text as published by upstream in ${path.basename(e.licenseFile)})`;
  } else {
    console.error(`notices.sh: no license file found for npm package ${key} — add ${overrideFile}`);
    process.exit(1);
  }
  const src = e.repository || e.url || '';
  const head = `${name}@${version} — ${e.licenses}` + (src ? ` — ${src}` : '') + note;
  process.stdout.write(`${SEP}\n${head}\n${SEP}\n\n`);
  process.stdout.write(fs.readFileSync(file, 'utf8').replace(/\s+$/, '') + '\n\n\n');
  count++;
}
if (count < 14) { console.error(`notices.sh: only ${count} npm entries (< 14 direct prod deps) — broken run`); process.exit(1); }
console.error(`npm entries: ${count}`);
NODE

# ---------------------------------------------------------------------------
# Assemble. No timestamps anywhere — reruns on the same dependency set are
# byte-identical.
# ---------------------------------------------------------------------------
{
  cat <<'HDR'
THIRD-PARTY LICENSES for xchats
================================================================================

This file aggregates the verbatim license and notice texts of every
third-party component distributed with xchats:

  SECTION 1 — Go modules statically linked into the backend binary
              (backend/cmd/xchats, module github.com/yerassyldanay/xchats/backend)
  SECTION 2 — npm packages bundled into the built frontend (production
              dependency tree of frontend/, direct and transitive)

Generated by scripts/notices.sh — DO NOT EDIT BY HAND; regenerate with
`make notices` (pinned tools: go-licenses/v2@v2.0.1,
license-checker-rseidelsohn@4.4.2). The dependency inventory tables and
obligations summary live in THIRD_PARTY_NOTICES.md. xchats itself is
licensed under AGPL-3.0-only (see LICENSE). Complete corresponding source
for each release (repo + vendored Go dependency source) is attached to the
matching GitHub Release as xchats-<tag>-corresponding-source.tar.gz.

HDR
  printf 'SECTION 1 — GO MODULES (%s entries)\n\n' "$GO_COUNT"
  cat "$TMP/section-go.txt"
  printf 'SECTION 2 — NPM PACKAGES\n\n'
  cat "$TMP/section-npm.txt"
} > "$OUT"

echo "notices.sh: wrote $OUT ($(wc -c < "$OUT") bytes, $GO_COUNT Go entries)"
