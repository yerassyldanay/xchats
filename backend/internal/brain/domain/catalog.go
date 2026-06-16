package domain

// ResolveAssets validates the refs the model picked against the snapshot's media
// catalog. Unknown/hallucinated refs are returned separately so the caller can
// drop and log them; the result is capped at 3 (docs/02 · post-processing step 3).
func (s *Snapshot) ResolveAssets(refs []string) (resolved []ResolvedAsset, unknown []string) {
	byRef := make(map[string]Asset, len(s.Assets))
	for _, a := range s.Assets {
		byRef[a.Ref] = a
	}
	for _, ref := range refs {
		a, ok := byRef[ref]
		if !ok {
			unknown = append(unknown, ref)
			continue
		}
		if len(resolved) >= 3 {
			break
		}
		resolved = append(resolved, ResolvedAsset{Ref: a.Ref, Kind: a.Kind, URL: a.URL})
	}
	return resolved, unknown
}
