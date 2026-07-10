package backup

// ExpandIncrementalChain adds ancestor version IDs required to restore kept incrementals.
func ExpandIncrementalChain(versions []VersionRef, keep map[string]bool) {
	byID := make(map[string]VersionRef, len(versions))
	for _, v := range versions {
		byID[v.ID] = v
	}
	for id := range keep {
		walkChain(id, byID, keep)
	}
}

type VersionRef struct {
	ID         string
	PathOnDisk string
}

func walkChain(id string, byID map[string]VersionRef, keep map[string]bool) {
	v, ok := byID[id]
	if !ok || v.PathOnDisk == "" {
		return
	}
	m, err := ReadFileManifest(v.PathOnDisk)
	if err != nil || m.BaseVersionID == "" {
		return
	}
	if keep[m.BaseVersionID] {
		return
	}
	keep[m.BaseVersionID] = true
	walkChain(m.BaseVersionID, byID, keep)
}
