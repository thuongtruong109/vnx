package vnx

import "strconv"

// ProvinceRef is a lightweight province reference (code + name).
type ProvinceRef struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

// WardRef is a lightweight ward reference with optional district/province context.
type WardRef struct {
	Code         int    `json:"code"`
	Name         string `json:"name"`
	DistrictCode int    `json:"district_code,omitempty"`
	DistrictName string `json:"district_name,omitempty"`
	ProvinceCode int    `json:"province_code,omitempty"`
	ProvinceName string `json:"province_name,omitempty"`
}

// ProvinceOldToNewResult is the result of resolving a v1 province → v2.
type ProvinceOldToNewResult struct {
	Found      bool         `json:"found"`
	V2Province *ProvinceRef `json:"v2_province,omitempty"`
	V2ID       string       `json:"v2_id,omitempty"`
}

// ProvinceNewToOldResult is the result of resolving a v2 province → v1 list.
type ProvinceNewToOldResult struct {
	Found       bool          `json:"found"`
	V2Province  *ProvinceRef  `json:"v2_province,omitempty"`
	V2ID        string        `json:"v2_id,omitempty"`
	V1Provinces []ProvinceRef `json:"v1_provinces"`
}

// WardOldToNewResult is the result of resolving a v1 ward → v2.
type WardOldToNewResult struct {
	Found   bool      `json:"found"`
	V2Ward  *WardRef  `json:"v2_ward,omitempty"`
	V1Wards []WardRef `json:"v1_wards"`
}

// WardNewToOldResult is the result of resolving a v2 ward → v1 list.
type WardNewToOldResult struct {
	Found   bool      `json:"found"`
	V2Ward  *WardRef  `json:"v2_ward,omitempty"`
	V1Wards []WardRef `json:"v1_wards"`
}

// OldToNewResult is the combined response from [ResolveOldToNew].
type OldToNewResult struct {
	Province *ProvinceOldToNewResult `json:"province,omitempty"`
	Ward     *WardOldToNewResult     `json:"ward,omitempty"`
}

// NewToOldResult is the combined response from [ResolveNewToOld].
type NewToOldResult struct {
	Province *ProvinceNewToOldResult `json:"province,omitempty"`
	Ward     *WardNewToOldResult     `json:"ward,omitempty"`
}

// ResolveOldToNew maps pre-2025 (v1) province and/or ward identifiers to their
// post-2025 (v2) equivalents. Pass an empty string to skip a lookup.
func ResolveOldToNew(store *Store, provinceQ, wardQ string) OldToNewResult {
	var result OldToNewResult
	if provinceQ != "" {
		result.Province = resolveProvinceOldToNew(store, provinceQ)
	}
	if wardQ != "" {
		result.Ward = resolveWardOldToNew(store, wardQ)
	}
	return result
}

// ResolveNewToOld maps post-2025 (v2) province and/or ward identifiers to
// their pre-2025 (v1) equivalents. Pass an empty string to skip a lookup.
func ResolveNewToOld(store *Store, provinceQ, wardQ string) NewToOldResult {
	var result NewToOldResult
	if provinceQ != "" {
		result.Province = resolveProvinceNewToOld(store, provinceQ)
	}
	if wardQ != "" {
		result.Ward = resolveWardNewToOld(store, wardQ)
	}
	return result
}

// -----------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------

func resolveProvinceOldToNew(s *Store, query string) *ProvinceOldToNewResult {
	var entry *ProvinceMapEntry
	if code, err := strconv.Atoi(query); err == nil {
		entry = s.ProvinceByV1Code[code]
	}
	if entry == nil {
		entry = s.ProvinceByV1Name[normName(query)]
	}
	if entry == nil {
		return &ProvinceOldToNewResult{Found: false}
	}
	return &ProvinceOldToNewResult{
		Found: true,
		V2Province: &ProvinceRef{
			Code: entry.V2Code,
			Name: entry.V2Name,
		},
		V2ID: entry.V2ID,
	}
}

func resolveProvinceNewToOld(s *Store, query string) *ProvinceNewToOldResult {
	var entry *ProvinceMapEntry
	if code, err := strconv.Atoi(query); err == nil {
		entry = s.ProvinceByV2Code[code]
	}
	if entry == nil {
		entry = s.ProvinceByV2Name[normName(query)]
	}
	if entry == nil {
		return &ProvinceNewToOldResult{Found: false, V1Provinces: []ProvinceRef{}}
	}
	old := make([]ProvinceRef, 0, len(entry.V1Codes))
	for i, code := range entry.V1Codes {
		name := ""
		if i < len(entry.V1Names) {
			name = entry.V1Names[i]
		}
		old = append(old, ProvinceRef{Code: code, Name: name})
	}
	return &ProvinceNewToOldResult{
		Found: true,
		V2Province: &ProvinceRef{
			Code: entry.V2Code,
			Name: entry.V2Name,
		},
		V2ID:        entry.V2ID,
		V1Provinces: old,
	}
}

func resolveWardOldToNew(s *Store, query string) *WardOldToNewResult {
	var entry *WardMapEntry
	if code, err := strconv.Atoi(query); err == nil {
		entry = s.WardByV1Code[code]
	}
	if entry == nil {
		entry = s.WardByV1Name[normName(query)]
	}
	if entry == nil {
		return &WardOldToNewResult{Found: false, V1Wards: []WardRef{}}
	}
	v1 := make([]WardRef, 0, len(entry.V1Wards))
	for _, ow := range entry.V1Wards {
		v1 = append(v1, WardRef{
			Code:         ow.Code,
			Name:         ow.Name,
			DistrictCode: ow.DistrictCode,
			DistrictName: ow.DistrictName,
			ProvinceCode: ow.ProvinceCode,
			ProvinceName: ow.ProvinceName,
		})
	}
	return &WardOldToNewResult{
		Found: true,
		V2Ward: &WardRef{
			Code:         entry.V2Code,
			Name:         entry.V2Name,
			ProvinceCode: entry.V2ProvinceCode,
		},
		V1Wards: v1,
	}
}

func resolveWardNewToOld(s *Store, query string) *WardNewToOldResult {
	var entry *WardMapEntry
	if code, err := strconv.Atoi(query); err == nil {
		entry = s.WardByV2Code[code]
	}
	if entry == nil {
		entry = s.WardByV2Name[normName(query)]
	}
	if entry == nil {
		return &WardNewToOldResult{Found: false, V1Wards: []WardRef{}}
	}
	v1 := make([]WardRef, 0, len(entry.V1Wards))
	for _, ow := range entry.V1Wards {
		v1 = append(v1, WardRef{
			Code:         ow.Code,
			Name:         ow.Name,
			DistrictCode: ow.DistrictCode,
			DistrictName: ow.DistrictName,
			ProvinceCode: ow.ProvinceCode,
			ProvinceName: ow.ProvinceName,
		})
	}
	return &WardNewToOldResult{
		Found: true,
		V2Ward: &WardRef{
			Code:         entry.V2Code,
			Name:         entry.V2Name,
			ProvinceCode: entry.V2ProvinceCode,
		},
		V1Wards: v1,
	}
}
