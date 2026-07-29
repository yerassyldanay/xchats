// Command harness is the eval playground's own tool: render turns a scenario's data.yaml
// into a real prompt + promptfoo config; judge grades promptfoo's raw answers against the
// scenario's fact/media catalog (token resolution, injection, fail-closed); report writes
// the human-readable run summary; extract runs Eval 1 (file -> extracted information)
// via direct OpenRouter calls, graded against extract/cases.yaml. It imports nothing from
// backend/ — this is a free-standing module on purpose (see evals/PLAYGROUND.md).
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "render":
		err = cmdRender(os.Args[2:])
	case "judge":
		err = cmdJudge(os.Args[2:])
	case "judge-llm":
		err = cmdJudgeLLM(os.Args[2:])
	case "rewrite-lang":
		err = cmdRewriteLang(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "run":
		_, err = cmdRun(os.Args[2:])
	case "retry":
		err = cmdRetry(os.Args[2:])
	case "extract":
		_, err = cmdExtract(os.Args[2:])
	case "html":
		err = cmdHTML(os.Args[2:])
	case "blind-export":
		err = cmdBlindExport(os.Args[2:])
	case "blind-report":
		err = cmdBlindReport(os.Args[2:])
	case "launch":
		err = cmdLaunch(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "schema-parity":
		err = cmdSchemaParity(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "harness:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: harness <command> [flags]

commands:
  render -scenario <dir> [-models m1,m2] [-models-file path]
                                       build generated/prompt.txt + catalog.json + promptfooconfig.yaml
  judge  -scenario <dir> -run <dir>   grade a run's results.json against the scenario's catalog
  judge-llm -scenario <dir> -run <dir> [-judge-model id] [-force]
                                       OPTIONAL, separate semantic-judge pass over an
                                       already-judged run: for every TestCase.llm_checks
                                       claim on a ContractPass row, asks a pinned cheap
                                       model one binary question and writes the result
                                       back onto judged.json. Never changes ContractPass/
                                       ModelBehaviorPass — the plain judge command's
                                       output is byte-identical whether or not this ever
                                       runs. Makes real, billed model calls; caches per
                                       claim, so a re-run only bills for new/changed rows
  rewrite-lang -scenario <dir> -run <dir> [-rewrite-model id] [-force]
                                       OPTIONAL language safety net over an already-
                                       judged run: for every ContractPass row, compares
                                       the customer message's language (detectLang) to
                                       the reply's actual language (classifyText on the
                                       injected text); on a mismatch, asks a cheap model
                                       to rewrite ONLY reply_text (every other field
                                       copied verbatim, placeholders preserved
                                       byte-for-byte), re-judges the rewrite from
                                       scratch, and keeps it only if it passes contract.
                                       Zero cost on the happy path (language already
                                       matches). Makes real, billed model calls only on a
                                       mismatch; -force re-checks rows already evaluated
  report -run <dir>                   write runs/<id>/SUMMARY.md from judge verdicts
  run    -scenario <dir>[,<dir>...] | -all   [-models m1,m2] [-models-file path] [-expect-calls N]
                                       render -> promptfoo eval -> judge -> report; prints the
                                       resolved (tests x models) call count before spending
                                       anything, and -expect-calls hard-fails if it doesn't match
  retry  -scenario <dir> -run <parent-dir> -expect-retry-calls N [-models-file path]
                                       repair a run's unparseable/empty rows by
                                       retrying each once, as a NEW derivative run — the
                                       parent is never modified; prints the resolved
                                       retry-candidate row count before spending
                                       anything, refuses if -expect-retry-calls doesn't
                                       match, and refuses to create a SECOND derivative
                                       for the same (parent, retry-config, code) unless
                                       -force-new-derivative is passed
  extract -cases <file> [-case <id>] [-models m1,m2] [-prompt name@vN] [-record]
                                       Eval 1: file -> extracted information (direct
                                       OpenRouter calls, no promptfoo); writes EXTRACT.md
  html   -run <dir>                   write runs/<id>/index.html (also auto-written,
                                       best-effort, at the end of run/extract/report)
  blind-export -run <dir> -out <dir> [-force] [-seed N]
                                       finalist-stage language review: strip prompt-
                                       variant/model identity from a judged run's
                                       ContractPass answers, shuffle, and write a
                                       reviewer-facing review.csv (blank kk/ru/mixed/
                                       unclear label column) plus a withheld mapping
                                       file; also writes ROUTING_ACCURACY.md immediately
                                       (needs no human review)
  blind-report -review <file> -mapping <file> [-out <file>]
                                       ingest a filled-in review.csv + its mapping file
                                       and write BLIND_REPORT.md: declared reply_language
                                       vs. the blinded human label, reported separately
  launch -all [-no-cache] [-models-file path] [-expect-calls N]
                                       umbrella over run+extract: mints one launch id,
                                       writes runs/launches/<id>.json BEFORE any billed
                                       call (planned families, combined pre-flight call
                                       count), then runs both families under that id so
                                       the eval comparison UI groups them as one launch
  export [-run <dir> | -all]          regenerate executions.json (per run) and runs.json
                                       (the launches list) — fatal on the first error,
                                       unlike the best-effort auto-write at the end of
                                       run/extract/report/html; the one command a fresh
                                       clone needs before the eval comparison UI has
                                       anything to show. -all also (re)writes
                                       runs/catalog.json — every scenario's and
                                       extraction case's requirements, resolved to real
                                       values, no billed calls; the read-only
                                       requirements review at /evals/catalog
  schema-parity -db <DATABASE_URL> [-mode transitional|strict]
                                       OPT-IN, explicit: compares the seven approved-KB
                                       production tables + kbd_materials against the
                                       manifest derived from internal/kbfixture's struct
                                       tags (cross-checked against backend/aiprompt's
                                       media registry — see internal/schemamanifest).
                                       -mode transitional (default) tolerates the named
                                       legacy columns/tables migration 0011 leaves in
                                       place; -mode strict allows nothing beyond the
                                       manifest (the state after migration 0012). Prints
                                       every diff and exits nonzero if any remain; never
                                       writes to the database`)
}
