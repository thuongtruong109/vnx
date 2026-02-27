package vnx

// ListRegions returns all active provinces grouped by their region name.
func ListRegions(store *Store) map[string][]Province {
	grouped := make(map[string][]Province)
	for _, p := range store.Provinces {
		grouped[p.Region] = append(grouped[p.Region], p)
	}
	return grouped
}
