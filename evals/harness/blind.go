package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xchats-evals-harness/internal/provenance"
)

// cmdBlindExport implements `harness blind-export` — the finalist-stage replacement for
// relying on looksKazakh alone: strips prompt-variant and model identity from every
// ContractPass answer in a judged run, shuffles them, and writes a reviewer-facing CSV
// (message + reply text + a blank label column) plus a SEPARATE, loudly-named mapping
// file the reviewer must never see. Also writes ROUTING_ACCURACY.md immediately — that
// metric needs no human review at all (see computeRoutingAccuracy).
func cmdBlindExport(args []string) error {
	fs := flag.NewFlagSet("blind-export", flag.ExitOnError)
	runDir := fs.String("run", "", "path to the run directory (contains *.judged.json)")
	outDir := fs.String("out", "", "directory to write review.csv + the mapping file into")
	force := fs.Bool("force", false, "overwrite an existing -out dir's review.csv (default: refuse, to protect an in-progress review)")
	seed := fs.Int64("seed", 0, "PRNG seed for shuffling (0 = time-based; pass a fixed nonzero value for a reproducible export, e.g. in a test)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runDir == "" || *outDir == "" {
		return fmt.Errorf("blind-export: -run and -out are both required")
	}

	if !*force {
		if _, err := os.Stat(filepath.Join(*outDir, "review.csv")); err == nil {
			return fmt.Errorf("blind-export: %s already exists — refusing to overwrite (a reviewer may have an in-progress review against it); pass -force to regenerate anyway", filepath.Join(*outDir, "review.csv"))
		}
	}

	judgedFiles, err := filepath.Glob(filepath.Join(*runDir, "*.judged.json"))
	if err != nil {
		return err
	}
	if len(judgedFiles) == 0 {
		return fmt.Errorf("blind-export: no *.judged.json in %s (did you run judge/run first?)", *runDir)
	}
	sort.Strings(judgedFiles)

	var entries []BlindMappingEntry
	var rows []BlindReviewRow
	excluded := 0
	for _, f := range judgedFiles {
		var jr JudgedRun
		if err := readJSON(f, &jr); err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		for _, v := range jr.Verdicts {
			if !v.ContractPass {
				excluded++ // nothing here is a meaningful language-quality judgment
				continue
			}
			declared, derr := declaredReplyLanguage(v.RawOutput)
			if derr != nil {
				// ContractPass=true guarantees this parses and has a valid
				// reply_language field (see judge.go's ContractFields) — a failure
				// here means judge.go and this command have silently drifted apart,
				// worth failing loudly over rather than exporting a broken row.
				return fmt.Errorf("blind-export: %s test %s: ContractPass=true but could not re-extract reply_language: %w", jr.Scenario, v.TestID, derr)
			}
			entries = append(entries, BlindMappingEntry{
				Scenario: jr.Scenario, TestID: v.TestID, Model: v.Model, DeclaredReplyLanguage: declared,
			})
			rows = append(rows, BlindReviewRow{Message: v.Message, ReplyText: v.InjectedText})
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("blind-export: no ContractPass verdicts in %s to review (%d excluded)", *runDir, excluded)
	}

	seedVal := *seed
	if seedVal == 0 {
		seedVal = time.Now().UnixNano()
	}
	rnd := rand.New(rand.NewSource(seedVal))
	rnd.Shuffle(len(rows), func(i, j int) {
		rows[i], rows[j] = rows[j], rows[i]
		entries[i], entries[j] = entries[j], entries[i]
	})

	idWidth := len(fmt.Sprintf("%d", len(rows)))
	mapping := BlindMappingFile{RunDir: *runDir, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Excluded: excluded}
	for i := range rows {
		id := fmt.Sprintf("R%0*d", idWidth, i+1)
		rows[i].OpaqueID = id
		entries[i].OpaqueID = id
		mapping.Entries = append(mapping.Entries, entries[i])
	}
	mapping.ReviewSHA256 = reviewContentHash(rows)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	reviewPath := filepath.Join(*outDir, "review.csv")
	if err := writeBlindReviewCSV(reviewPath, rows); err != nil {
		return err
	}
	mappingPath := filepath.Join(*outDir, "mapping.DO-NOT-SHARE-WITH-REVIEWER.json")
	if err := writeJSON(mappingPath, mapping); err != nil {
		return err
	}
	routingPath := filepath.Join(*outDir, "ROUTING_ACCURACY.md")
	if err := writeRoutingAccuracyReport(routingPath, judgedFiles, *runDir); err != nil {
		return err
	}

	fmt.Printf("blind-export: %d rows for review (%d excluded, not ContractPass), shuffled with seed %d\n", len(rows), excluded, seedVal)
	fmt.Printf("  reviewer-facing:            %s\n", reviewPath)
	fmt.Printf("  WITHHOLD FROM THE REVIEWER: %s\n", mappingPath)
	fmt.Printf("  routing accuracy (no review needed): %s\n", routingPath)
	return nil
}

// declaredReplyLanguage re-extracts a verdict's raw reply_language value — the ACTUAL
// declared string, not just whether it matched an expectation (judge.go's Verdict only
// keeps the latter, LanguageFieldOK). Re-parsing here (rather than adding a redundant
// field to every Verdict, multiplied across every -repeats copy) reuses judge.go's own
// parseModelJSON so the two can never disagree about what "parses" means.
func declaredReplyLanguage(raw string) (string, error) {
	obj, ok := parseModelJSON(raw)
	if !ok {
		return "", fmt.Errorf("could not parse JSON output")
	}
	lang, ok := obj["reply_language"].(string)
	if !ok {
		return "", fmt.Errorf("reply_language field missing or not a string")
	}
	return lang, nil
}

// reviewContentHash hashes the (opaque_id, message, reply_text) columns, in order — NOT
// Label, which is expected to change between export and report. See
// BlindMappingFile.ReviewSHA256's doc comment for why this, not opaque-id-set membership
// alone, is the real "these two files belong together" guarantee.
func reviewContentHash(rows []BlindReviewRow) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.OpaqueID)
		b.WriteByte(0)
		b.WriteString(r.Message)
		b.WriteByte(0)
		b.WriteString(r.ReplyText)
		b.WriteByte(0x1e) // record separator
	}
	return provenance.SHA256Bytes([]byte(b.String()))
}

func writeBlindReviewCSV(path string, rows []BlindReviewRow) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(blindReviewCSVHeader); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{r.OpaqueID, r.Message, r.ReplyText, r.Label}); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return provenance.AtomicWriteFile(path, buf.Bytes(), 0o644)
}

func readBlindReviewCSV(path string) ([]BlindReviewRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	header := records[0]
	if len(header) != len(blindReviewCSVHeader) {
		return nil, fmt.Errorf("%s: want %d columns %v, got %d: %v", path, len(blindReviewCSVHeader), blindReviewCSVHeader, len(header), header)
	}
	for i, want := range blindReviewCSVHeader {
		if header[i] != want {
			return nil, fmt.Errorf("%s: column %d is %q, want %q — has the file been hand-edited beyond the label column?", path, i, header[i], want)
		}
	}
	rows := make([]BlindReviewRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		rows = append(rows, BlindReviewRow{OpaqueID: rec[0], Message: rec[1], ReplyText: rec[2], Label: strings.TrimSpace(rec[3])})
	}
	return rows, nil
}

// writeRoutingAccuracyReport computes and writes routing accuracy for every scenario
// represented in judgedFiles — a pure function of each scenario's resolved test set
// (read from this run's OWN snapshot, matching judge.go's snapshot-over-live preference,
// so re-generating this report later still grades against what the run actually saw),
// never of any model output. Runs from before provenance snapshotting existed (no
// runs/<id>/snapshots/<scenario>/) report as unavailable for that scenario rather than
// silently falling back to a live scenarios/*/generated/ that may have since changed.
func writeRoutingAccuracyReport(path string, judgedFiles []string, runDir string) error {
	var b strings.Builder
	b.WriteString("# Routing accuracy\n\n")
	b.WriteString("detectLang(message, history) vs. each test's own hand-authored `language` field — computed directly from the resolved test set, independent of any model output or human review (see blind_types.go's computeRoutingAccuracy). This is its OWN metric: never combined with declared reply_language or the blinded prose-language label (BLIND_REPORT.md, written by blind-report) into one pass/fail.\n\n")

	var scenarios []string
	seen := map[string]bool{}
	for _, f := range judgedFiles {
		var jr JudgedRun
		if err := readJSON(f, &jr); err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if !seen[jr.Scenario] {
			seen[jr.Scenario] = true
			scenarios = append(scenarios, jr.Scenario)
		}
	}
	sort.Strings(scenarios)

	for _, scenario := range scenarios {
		snapDir, ok := provenance.SnapshotDirFor(runDir, scenario)
		if !ok {
			fmt.Fprintf(&b, "## %s\n\nNo resolved_tests.json snapshot found for this scenario (a run from before provenance snapshotting existed) — routing accuracy unavailable.\n\n", scenario)
			continue
		}
		var resolved ResolvedTests
		if err := readJSON(filepath.Join(snapDir, "resolved_tests.json"), &resolved); err != nil {
			return fmt.Errorf("read %s resolved_tests.json: %w", scenario, err)
		}
		report := computeRoutingAccuracy(resolved.Tests)
		fmt.Fprintf(&b, "## %s\n\n%d/%d (%.0f%%) correct", scenario, report.Correct, report.Total, pct(report.Correct, report.Total))
		if report.SkippedNoLanguage > 0 {
			fmt.Fprintf(&b, " · %d test(s) skipped (no `language` annotation)", report.SkippedNoLanguage)
		}
		b.WriteString("\n\n")
		if len(report.Mismatches) > 0 {
			b.WriteString("Mismatches:\n\n")
			for _, m := range report.Mismatches {
				fmt.Fprintf(&b, "- `%s`: detectLang said %q, test expects %q — %q\n", m.TestID, m.Got, m.Want, m.Message)
			}
			b.WriteString("\n")
		}
	}
	return provenance.AtomicWriteFile(path, []byte(b.String()), 0o644)
}

// cmdBlindReport implements `harness blind-report` — ingests a FILLED-IN review.csv plus
// its matching mapping file and reports the remaining two signals (declared
// reply_language, blinded prose-language label) as explicitly separate sections, never
// collapsed into one pass/fail.
func cmdBlindReport(args []string) error {
	fs := flag.NewFlagSet("blind-report", flag.ExitOnError)
	reviewPath := fs.String("review", "", "path to the filled-in review.csv")
	mappingPath := fs.String("mapping", "", "path to the matching mapping.DO-NOT-SHARE-WITH-REVIEWER.json from the same blind-export run")
	outPath := fs.String("out", "", "where to write BLIND_REPORT.md (default: alongside -review)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *reviewPath == "" || *mappingPath == "" {
		return fmt.Errorf("blind-report: -review and -mapping are both required")
	}
	if *outPath == "" {
		*outPath = filepath.Join(filepath.Dir(*reviewPath), "BLIND_REPORT.md")
	}

	rows, err := readBlindReviewCSV(*reviewPath)
	if err != nil {
		return err
	}
	var mapping BlindMappingFile
	if err := readJSON(*mappingPath, &mapping); err != nil {
		return fmt.Errorf("read %s: %w", *mappingPath, err)
	}

	byID := map[string]BlindMappingEntry{}
	for _, e := range mapping.Entries {
		byID[e.OpaqueID] = e
	}
	// Row-ID sets must match — a count or membership mismatch means these two files
	// obviously don't belong together (a friendlier error than the hash check below for
	// the common case: wrong file passed, or a row deleted).
	if len(rows) != len(mapping.Entries) {
		return fmt.Errorf("blind-report: %s has %d rows but %s has %d entries — these must be the exact export/mapping pair from the same blind-export run", *reviewPath, len(rows), *mappingPath, len(mapping.Entries))
	}
	for _, r := range rows {
		if _, ok := byID[r.OpaqueID]; !ok {
			return fmt.Errorf("blind-report: row %q in %s has no matching entry in %s — mismatched export/mapping pair", r.OpaqueID, *reviewPath, *mappingPath)
		}
	}
	// The authoritative check: opaque ids alone are NOT globally unique (two same-sized
	// exports from different runs both produce the identical id set "R1".."RN", since
	// ids are assigned sequentially per export) — a content hash over (opaque_id,
	// message, reply_text) is what actually proves this review.csv is the SAME export
	// mapping.json was written for, unmodified beyond the label column a reviewer fills in.
	if got := reviewContentHash(rows); got != mapping.ReviewSHA256 {
		return fmt.Errorf("blind-report: %s does not match the content hash recorded in %s — either a different export's mapping file was passed, or review.csv was hand-edited beyond the label column", *reviewPath, *mappingPath)
	}

	report := buildBlindReport(rows, byID)
	if err := provenance.AtomicWriteFile(*outPath, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Printf("blind-report: wrote %s\n", *outPath)
	return nil
}

// validBlindLabels is the reviewer's allowed vocabulary — kk / ru / mixed / unclear.
var validBlindLabels = map[string]bool{"kk": true, "ru": true, "mixed": true, "unclear": true}

// buildBlindReport computes the two post-review signals. A blank Label is "not yet
// reviewed" — a distinct state from any real label, mirroring viewmodel.go's
// ScoreNotRun precedent — so a partial review can never masquerade as a complete one; a
// non-blank but unrecognized value (a reviewer typo) is counted in the label
// distribution but excluded from the agreement comparison, and never silently coerced
// into kk or ru.
func buildBlindReport(rows []BlindReviewRow, byID map[string]BlindMappingEntry) string {
	var b strings.Builder
	b.WriteString("# Blinded review report\n\n")
	b.WriteString("Two of the three tracked signals — routing accuracy is its own file (ROUTING_ACCURACY.md, written by blind-export; needs no human review). Below: the model's own declared reply_language field, and the blinded human prose-language label, plus how often they agree. Never collapsed into one pass/fail.\n\n")

	labeled := 0
	labelCounts := map[string]int{}
	declaredCounts := map[string]int{}
	agree, compared := 0, 0
	var disagreements []string
	for _, r := range rows {
		entry := byID[r.OpaqueID]
		declaredCounts[entry.DeclaredReplyLanguage]++
		if r.Label == "" {
			continue // not yet reviewed — excluded from every stat below, not counted as a miss
		}
		labeled++
		labelCounts[r.Label]++
		if !validBlindLabels[r.Label] {
			continue // logged in the distribution above, but not a recognized value to compare
		}
		compared++
		if r.Label == entry.DeclaredReplyLanguage {
			agree++
		} else {
			disagreements = append(disagreements, fmt.Sprintf("- `%s` (%s / %s): declared %q, blinded human label %q — %q",
				r.OpaqueID, entry.Scenario, entry.Model, entry.DeclaredReplyLanguage, r.Label, r.ReplyText))
		}
	}

	fmt.Fprintf(&b, "## Review completeness\n\n%d of %d rows labeled (%.0f%%)\n\n", labeled, len(rows), pct(labeled, len(rows)))

	b.WriteString("## Declared reply_language (model's own field)\n\n")
	for _, lang := range sortedCountKeys(declaredCounts) {
		fmt.Fprintf(&b, "- %s: %d\n", lang, declaredCounts[lang])
	}
	b.WriteString("\n")

	b.WriteString("## Blinded prose-language label (human, blind to model/variant)\n\n")
	for _, lang := range sortedCountKeys(labelCounts) {
		fmt.Fprintf(&b, "- %s: %d\n", lang, labelCounts[lang])
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## Declared vs. blinded agreement\n\n%d/%d (%.0f%%) agree — over labeled rows with a recognized kk/ru/mixed/unclear value; \"mixed\"/\"unclear\" count as their own non-match category, never coerced to kk or ru.\n\n",
		agree, compared, pct(agree, compared))
	if len(disagreements) > 0 {
		b.WriteString("Disagreements:\n\n")
		for _, d := range disagreements {
			b.WriteString(d + "\n")
		}
	}
	return b.String()
}

func sortedCountKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
