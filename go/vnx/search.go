package vnx

import "strings"

// SearchEntry is a single result item from [Search].
type SearchEntry struct {
	Type         string      `json:"type"`
	ProvinceID   string      `json:"province_id,omitempty"`
	Name         string      `json:"name"`
	DivisionType string      `json:"division_type"`
	Status       AdminStatus `json:"status,omitempty"`
	MergedInto   string      `json:"merged_into,omitempty"`
}

// SearchResult is the response from [Search].
type SearchResult struct {
	Query   string        `json:"query"`
	Total   int           `json:"total"`
	Results []SearchEntry `json:"results"`
}

// Search performs a case-insensitive substring search across provinces,
// districts, and wards.
//
// typ filters results to a specific entity type: "province", "district", or
// "ward". An empty string returns all types.
//
// Pass includeInactive=true to include merged/split/renamed units.
func Search(store *Store, query, typ string, includeInactive bool) SearchResult {
	q := strings.ToLower(query)
	var results []SearchEntry

	if typ == "" || typ == "province" {
		for _, p := range store.Provinces {
			if !includeInactive && !IsActive(p.Status) {
				continue
			}
			if strings.Contains(strings.ToLower(p.Name), q) {
				results = append(results, SearchEntry{
					Type:         "province",
					Name:         p.Name,
					DivisionType: p.DivisionType,
					Status:       p.Status,
					MergedInto:   p.MergedInto,
				})
			}
		}
	}

	if typ == "" || typ == "district" || typ == "ward" {
		for _, entry := range store.Addresses {
			for _, d := range entry.Districts {
				if (typ == "" || typ == "district") && strings.Contains(strings.ToLower(d.Name), q) {
					if includeInactive || IsActive(d.Status) {
						results = append(results, SearchEntry{
							Type:         "district",
							ProvinceID:   entry.ProvinceID,
							Name:         d.Name,
							DivisionType: d.DivisionType,
							Status:       d.Status,
							MergedInto:   d.MergedInto,
						})
					}
				}
				if typ == "" || typ == "ward" {
					for _, ward := range d.Wards {
						if strings.Contains(strings.ToLower(ward.Name), q) {
							if includeInactive || IsActive(ward.Status) {
								results = append(results, SearchEntry{
									Type:         "ward",
									ProvinceID:   entry.ProvinceID,
									Name:         ward.Name,
									DivisionType: ward.DivisionType,
									Status:       ward.Status,
									MergedInto:   ward.MergedInto,
								})
							}
						}
					}
				}
			}
		}
	}

	if results == nil {
		results = []SearchEntry{}
	}
	return SearchResult{
		Query:   q,
		Total:   len(results),
		Results: results,
	}
}
