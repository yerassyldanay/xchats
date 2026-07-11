package main

// RoutingMismatch is one test where detectLang's routing decision disagreed with the
// test's own hand-authored Language field.
type RoutingMismatch struct {
	TestID  string `json:"test_id"`
	Message string `json:"message"`
	Got     string `json:"got"`  // what detectLang(tc.Message, tc.History) returned
	Want    string `json:"want"` // tc.Language
}

// RoutingAccuracyReport is routing accuracy's own, separate metric — see
// computeRoutingAccuracy's doc comment for why it needs no live human review to compute.
type RoutingAccuracyReport struct {
	Total             int               `json:"total"`
	Correct           int               `json:"correct"`
	SkippedNoLanguage int               `json:"skipped_no_language"`
	Mismatches        []RoutingMismatch `json:"mismatches,omitempty"`
}

// computeRoutingAccuracy answers "did detectLang pick the frame a human would have,"
// WITHOUT waiting on a live human-labeling pass: TestCase.Language is already a human's
// a-priori judgment of the correct frame, recorded at test-authoring time, and the
// frames' own policy ("mixed language -> reply in Russian") makes it the same axis
// detectLang targets by construction — exactly what langdetect_scenario_test.go's
// TestDetectLang_AgreesWithV4CanarySplit already assumes for the hand-picked V4 split.
// This is a pure function of test definitions (never of a specific model's output), so
// it's the same one metric regardless of how many models or repetitions produced
// verdicts for a given test — deliberately NOT threaded through the blind-review
// export/report pipeline, which grades OUTPUT prose, a different question entirely.
//
// A test with no Language annotation (some scenario-only tests never set one) is
// excluded from the denominator and counted separately — never silently folded into
// either "correct" or "incorrect", which would misrepresent the real accuracy.
func computeRoutingAccuracy(tests []TestCase) RoutingAccuracyReport {
	var report RoutingAccuracyReport
	for _, tc := range tests {
		if tc.Language != "kk" && tc.Language != "ru" {
			report.SkippedNoLanguage++
			continue
		}
		report.Total++
		got := detectLang(tc.Message, tc.History)
		if got == tc.Language {
			report.Correct++
			continue
		}
		report.Mismatches = append(report.Mismatches, RoutingMismatch{
			TestID: tc.ID, Message: tc.Message, Got: got, Want: tc.Language,
		})
	}
	return report
}

// BlindReviewRow is one row of the reviewer-facing export (review.csv). Deliberately
// carries NOTHING beyond message + reply text: no model id, no prompt variant, no
// declared reply_language — showing any of those would anchor the reviewer's judgment
// away from an independent read of the prose itself, defeating the point of a blinded
// label. Label is blank on export; the reviewer fills in kk/ru/mixed/unclear by hand.
type BlindReviewRow struct {
	OpaqueID  string
	Message   string
	ReplyText string
	Label     string
}

// blindReviewCSVHeader is review.csv's column order — shared by the writer (blind-export)
// and reader (blind-report) so the two can never silently drift apart.
var blindReviewCSVHeader = []string{"opaque_id", "message", "reply_text", "label"}

// BlindMappingEntry is what's withheld from the reviewer: which real (scenario, test,
// model) an opaque row actually came from, plus the model's own declared reply_language
// — needed to compute the declared-vs-blinded-label metric once labels come back, but
// never shown to whoever is doing the blind labeling.
type BlindMappingEntry struct {
	OpaqueID              string `json:"opaque_id"`
	Scenario              string `json:"scenario"`
	TestID                string `json:"test_id"`
	Model                 string `json:"model"`
	DeclaredReplyLanguage string `json:"declared_reply_language"`
}

// BlindMappingFile is mapping.DO-NOT-SHARE-WITH-REVIEWER.json's shape — the filename
// itself is the actual safeguard (Go can't enforce who a file gets handed to), this
// struct just fixes what's in it.
type BlindMappingFile struct {
	RunDir      string `json:"run_dir"`
	GeneratedAt string `json:"generated_at"`
	Excluded    int    `json:"excluded_non_contract_pass"`
	// ReviewSHA256 hashes the (opaque_id, message, reply_text) columns of the exported
	// review.csv, in order — NOT the label column, which is expected to change once a
	// reviewer fills it in. blind-report recomputes this from whatever review.csv it's
	// handed and rejects a mismatch: opaque ids alone aren't enough (two SAME-SIZED
	// exports from different runs both produce the identical id set "R1".."RN", since
	// ids are assigned sequentially per export, not globally unique), and this also
	// catches a reviewer hand-editing anything beyond the label column.
	ReviewSHA256 string              `json:"review_sha256"`
	Entries      []BlindMappingEntry `json:"entries"`
}
