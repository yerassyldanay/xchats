package main

import "path/filepath"

// archivedScenarioInfo pairs a scenario's archived flag with its reason.
type archivedScenarioInfo struct {
	Archived bool
	Reason   string
}

// archivedModelIDs maps mf's ArchivedModels to (de-prefixed) id -> reason, normalized
// the same way filterProviders/providerModelKey already compare ids (orModelID) so a
// lookup by either a bare or "openrouter:"-prefixed id hits the same entry.
func archivedModelIDs(mf *ModelsFile) map[string]string {
	out := map[string]string{}
	for _, m := range mf.ArchivedModels {
		out[orModelID(m.ID)] = m.Reason
	}
	return out
}

// loadArchivedScenarios scans the CURRENT scenarios/*/scenario.yaml (relative to cwd,
// same glob run.go/launch.go/catalog.go use) and returns every archived scenario's
// Name -> info. Best-effort: an unreadable or missing scenario dir (e.g. one a
// historical run referenced that has since been deleted) is simply absent from the
// map, never an error — this is a display-time badge lookup, not a correctness gate.
//
// Resolved fresh from disk on every call rather than cached or baked into a run: per
// ScenarioConfig.Archived's doc comment, archival is a fact about TODAY's active
// roster, so a run from before a scenario was archived still shows today's badge, and
// un-archiving a scenario later immediately stops showing it on every past run's page.
func loadArchivedScenarios() map[string]archivedScenarioInfo {
	out := map[string]archivedScenarioInfo{}
	matches, err := filepath.Glob(filepath.Join("scenarios", "*", "scenario.yaml"))
	if err != nil {
		return out
	}
	for _, m := range matches {
		sc, err := loadScenario(filepath.Dir(m))
		if err != nil || !sc.Archived {
			continue
		}
		out[sc.Name] = archivedScenarioInfo{Archived: true, Reason: sc.ArchivedReason}
	}
	return out
}

// loadArchivedModels is loadArchivedScenarios' model-side counterpart: reads the
// CURRENT default models.yaml path (best-effort — absent/unreadable returns an empty
// map rather than failing an otherwise-successful HTML/index render) regardless of
// which models.yaml a historical run itself snapshotted, for the same "archival is a
// today fact, resolved at display time" reason.
func loadArchivedModels() map[string]string {
	mf, err := loadModels("models.yaml")
	if err != nil {
		return map[string]string{}
	}
	return archivedModelIDs(mf)
}
