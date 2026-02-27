package vnx

import "fmt"

// DistrictSummary is the response shape returned by [GetDistricts].
type DistrictSummary struct {
	Name          string      `json:"name"`
	DivisionType  string      `json:"division_type"`
	WardCount     int         `json:"ward_count"`
	Status        AdminStatus `json:"status,omitempty"`
	MergedInto    string      `json:"merged_into,omitempty"`
	MergedFrom    []string    `json:"merged_from,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	EndDate       string      `json:"end_date,omitempty"`
}

// ProvinceAtDepth builds a response object for the given province at the
// requested depth level.
//
//   - deep=1  — province fields only
//   - deep=2  — province + districts (no wards)
//   - deep=3  — province + districts + wards (no street/village details)
//   - deep=4  — province + districts + wards + street/village details
//
// includeInactive controls whether non-active districts/wards are included.
func ProvinceAtDepth(store *Store, p Province, depth int, includeInactive bool) any {
	if depth == 1 {
		return p
	}

	entry := store.AddressByProvince[p.ID]

	if depth == 2 {
		type resp struct {
			Province
			Districts []DistrictNoWards `json:"districts"`
		}
		r := resp{Province: p, Districts: []DistrictNoWards{}}
		if entry != nil {
			for _, d := range entry.Districts {
				if !includeInactive && !IsActive(d.Status) {
					continue
				}
				r.Districts = append(r.Districts, DistrictNoWards{
					Name:          d.Name,
					DivisionType:  d.DivisionType,
					Status:        d.Status,
					EffectiveDate: d.EffectiveDate,
					EndDate:       d.EndDate,
					MergedFrom:    d.MergedFrom,
					MergedInto:    d.MergedInto,
				})
			}
		}
		return r
	}

	if depth == 3 {
		type resp struct {
			Province
			Districts []DistrictWithWardsNoDetails `json:"districts"`
		}
		r := resp{Province: p, Districts: []DistrictWithWardsNoDetails{}}
		if entry != nil {
			for _, d := range entry.Districts {
				if !includeInactive && !IsActive(d.Status) {
					continue
				}
				wd := DistrictWithWardsNoDetails{
					Name:          d.Name,
					DivisionType:  d.DivisionType,
					Status:        d.Status,
					EffectiveDate: d.EffectiveDate,
					EndDate:       d.EndDate,
					MergedFrom:    d.MergedFrom,
					MergedInto:    d.MergedInto,
					Wards:         []WardNoDetails{},
				}
				for _, w := range d.Wards {
					if !includeInactive && !IsActive(w.Status) {
						continue
					}
					wd.Wards = append(wd.Wards, WardNoDetails{
						Name:          w.Name,
						DivisionType:  w.DivisionType,
						Code:          w.Code,
						Status:        w.Status,
						EffectiveDate: w.EffectiveDate,
						EndDate:       w.EndDate,
						MergedFrom:    w.MergedFrom,
						MergedInto:    w.MergedInto,
					})
				}
				r.Districts = append(r.Districts, wd)
			}
		}
		return r
	}

	// depth == 4: full detail
	type resp struct {
		Province
		Districts []District `json:"districts"`
	}
	r := resp{Province: p, Districts: []District{}}
	if entry != nil {
		for _, d := range entry.Districts {
			if !includeInactive && !IsActive(d.Status) {
				continue
			}
			if includeInactive {
				r.Districts = append(r.Districts, d)
			} else {
				filtered := District{
					Name:          d.Name,
					DivisionType:  d.DivisionType,
					Status:        d.Status,
					EffectiveDate: d.EffectiveDate,
					EndDate:       d.EndDate,
					MergedFrom:    d.MergedFrom,
					MergedInto:    d.MergedInto,
					Wards:         []Ward{},
				}
				for _, w := range d.Wards {
					if IsActive(w.Status) {
						filtered.Wards = append(filtered.Wards, w)
					}
				}
				r.Districts = append(r.Districts, filtered)
			}
		}
	}
	return r
}

// GetProvinces returns the list of provinces, optionally including inactive
// units, at the requested depth level (1–4, see [ProvinceAtDepth]).
func GetProvinces(store *Store, includeInactive bool, deep int) []any {
	results := make([]any, 0, len(store.Provinces))
	for _, p := range store.Provinces {
		if !includeInactive && !IsActive(p.Status) {
			continue
		}
		results = append(results, ProvinceAtDepth(store, p, deep, includeInactive))
	}
	return results
}

// GetProvince returns a single province by its string ID at the given depth.
// Returns an error when the province is not found.
func GetProvince(store *Store, id string, deep int, includeInactive bool) (any, error) {
	for _, p := range store.Provinces {
		if p.ID == id {
			return ProvinceAtDepth(store, p, deep, includeInactive), nil
		}
	}
	return nil, fmt.Errorf("province not found: %s", id)
}

// GetProvinceDetail returns full province info at the given depth (defaults to
// 4 when deep < 1 or deep > 4). Returns an error when the province is not found.
func GetProvinceDetail(store *Store, id string, deep int, includeInactive bool) (any, error) {
	if deep < 1 || deep > 4 {
		deep = 4
	}
	for i := range store.Provinces {
		if store.Provinces[i].ID == id {
			return ProvinceAtDepth(store, store.Provinces[i], deep, includeInactive), nil
		}
	}
	return nil, fmt.Errorf("province not found: %s", id)
}

// GetDistricts returns all districts for a province, summarised (no wards).
// Pass includeInactive=true to include merged/split/renamed units.
// Returns an error when the province address data is not found.
func GetDistricts(store *Store, provinceID string, includeInactive bool) ([]DistrictSummary, error) {
	entry, ok := store.AddressByProvince[provinceID]
	if !ok {
		return nil, fmt.Errorf("province address data not found: %s", provinceID)
	}

	result := make([]DistrictSummary, 0, len(entry.Districts))
	for _, d := range entry.Districts {
		if !includeInactive && !IsActive(d.Status) {
			continue
		}
		result = append(result, DistrictSummary{
			Name:          d.Name,
			DivisionType:  d.DivisionType,
			WardCount:     len(d.Wards),
			Status:        d.Status,
			MergedInto:    d.MergedInto,
			MergedFrom:    d.MergedFrom,
			EffectiveDate: d.EffectiveDate,
			EndDate:       d.EndDate,
		})
	}
	return result, nil
}

// GetDistrict returns a single district (with all its wards) by name.
// Returns an error when the province or district is not found.
func GetDistrict(store *Store, provinceID, districtName string) (*District, error) {
	entry, ok := store.AddressByProvince[provinceID]
	if !ok {
		return nil, fmt.Errorf("province address data not found: %s", provinceID)
	}
	for i := range entry.Districts {
		if entry.Districts[i].Name == districtName {
			d := entry.Districts[i]
			return &d, nil
		}
	}
	return nil, fmt.Errorf("district not found: %s", districtName)
}

// GetWards returns all wards for a district. Pass includeInactive=true to
// include merged/split/renamed units.
// Returns an error when the province or district is not found.
func GetWards(store *Store, provinceID, districtName string, includeInactive bool) ([]Ward, error) {
	entry, ok := store.AddressByProvince[provinceID]
	if !ok {
		return nil, fmt.Errorf("province address data not found: %s", provinceID)
	}
	for _, d := range entry.Districts {
		if d.Name == districtName {
			if includeInactive {
				return d.Wards, nil
			}
			active := make([]Ward, 0, len(d.Wards))
			for _, w := range d.Wards {
				if IsActive(w.Status) {
					active = append(active, w)
				}
			}
			return active, nil
		}
	}
	return nil, fmt.Errorf("district not found: %s", districtName)
}

// GetWard returns a single ward by name with full street/village details.
// Returns an error when the province, district, or ward is not found.
func GetWard(store *Store, provinceID, districtName, wardName string) (*Ward, error) {
	entry, ok := store.AddressByProvince[provinceID]
	if !ok {
		return nil, fmt.Errorf("province address data not found: %s", provinceID)
	}
	for _, d := range entry.Districts {
		if d.Name == districtName {
			for i := range d.Wards {
				if d.Wards[i].Name == wardName {
					w := d.Wards[i]
					return &w, nil
				}
			}
			return nil, fmt.Errorf("ward not found: %s", wardName)
		}
	}
	return nil, fmt.Errorf("district not found: %s", districtName)
}
