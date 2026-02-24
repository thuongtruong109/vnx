package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"vnx-api/internal/loader"
	"vnx-api/internal/models"
)

// -----------------------------------------------------------------
// Shared response shapes
// -----------------------------------------------------------------

type provinceRef struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

type wardRef struct {
	Code         int    `json:"code"`
	Name         string `json:"name"`
	DistrictCode int    `json:"district_code,omitempty"`
	DistrictName string `json:"district_name,omitempty"`
	ProvinceCode int    `json:"province_code,omitempty"`
	ProvinceName string `json:"province_name,omitempty"`
}

// -----------------------------------------------------------------
// GET /api/resolve/old-to-new
//
// Query params (at least one required):
//   province  — old (v1) province name or numeric code
//   district  — old (v1) district name (optional)
//   ward      — old (v1) ward name or numeric code (optional)
//
// Response:
//   {
//     "query": { "province": "...", "district": "...", "ward": "..." },
//     "province": { v2 province entry or null },
//     "ward":     { v2 ward entry or null }
//   }
// -----------------------------------------------------------------
func (h *Handler) ResolveOldToNew(w http.ResponseWriter, r *http.Request) {
	provinceQ := r.URL.Query().Get("province")
	districtQ := r.URL.Query().Get("district")
	wardQ := r.URL.Query().Get("ward")

	if provinceQ == "" && wardQ == "" {
		writeError(w, http.StatusBadRequest, "at least one of 'province' or 'ward' is required")
		return
	}

	type response struct {
		Query    map[string]string      `json:"query"`
		Province *provinceResolution    `json:"province"`
		Ward     *wardNewResolution     `json:"ward"`
	}

	resp := response{
		Query: map[string]string{
			"province": provinceQ,
			"district": districtQ,
			"ward":     wardQ,
		},
	}

	// --- Resolve province ---
	if provinceQ != "" {
		resp.Province = resolveProvinceOldToNew(h.store, provinceQ)
	}

	// --- Resolve ward ---
	if wardQ != "" {
		resp.Ward = resolveWardOldToNew(h.store, wardQ)
	}

	writeJSON(w, http.StatusOK, resp)
}

// -----------------------------------------------------------------
// GET /api/resolve/new-to-old
//
// Query params (at least one required):
//   province  — new (v2) province name or numeric code
//   ward      — new (v2) ward name or numeric code
//
// Response:
//   {
//     "query": { "province": "...", "ward": "..." },
//     "province": { v2 province → list of old v1 provinces },
//     "ward":     { v2 ward → list of old v1 wards }
//   }
// -----------------------------------------------------------------
func (h *Handler) ResolveNewToOld(w http.ResponseWriter, r *http.Request) {
	provinceQ := r.URL.Query().Get("province")
	wardQ := r.URL.Query().Get("ward")

	if provinceQ == "" && wardQ == "" {
		writeError(w, http.StatusBadRequest, "at least one of 'province' or 'ward' is required")
		return
	}

	type response struct {
		Query    map[string]string      `json:"query"`
		Province *provinceOldResolution `json:"province"`
		Ward     *wardOldResolution     `json:"ward"`
	}

	resp := response{
		Query: map[string]string{
			"province": provinceQ,
			"ward":     wardQ,
		},
	}

	if provinceQ != "" {
		resp.Province = resolveProvinceNewToOld(h.store, provinceQ)
	}
	if wardQ != "" {
		resp.Ward = resolveWardNewToOld(h.store, wardQ)
	}

	writeJSON(w, http.StatusOK, resp)
}

// -----------------------------------------------------------------
// Internal resolution helpers
// -----------------------------------------------------------------

// provinceResolution is the result of an old-to-new province lookup.
type provinceResolution struct {
	Found      bool        `json:"found"`
	V2Province *provinceRef `json:"v2_province,omitempty"`
	V2ID       string      `json:"v2_id,omitempty"`
}

// provinceOldResolution is the result of a new-to-old province lookup.
type provinceOldResolution struct {
	Found       bool          `json:"found"`
	V2Province  *provinceRef  `json:"v2_province,omitempty"`
	V2ID        string        `json:"v2_id,omitempty"`
	V1Provinces []provinceRef `json:"v1_provinces"`
}

// wardNewResolution is the result of an old ward → new ward lookup.
type wardNewResolution struct {
	Found   bool     `json:"found"`
	V2Ward  *wardRef `json:"v2_ward,omitempty"`
	V1Wards []wardRef `json:"v1_wards"` // the specific old ward(s) that matched
}

// wardOldResolution is the result of a new ward → old wards lookup.
type wardOldResolution struct {
	Found   bool      `json:"found"`
	V2Ward  *wardRef  `json:"v2_ward,omitempty"`
	V1Wards []wardRef `json:"v1_wards"`
}

// resolveProvinceOldToNew looks up a v1 province (by code or name) and returns
// the v2 province it became.
func resolveProvinceOldToNew(s *loader.Store, query string) *provinceResolution {
	var entry *models.ProvinceMapEntry

	if code, err := strconv.Atoi(query); err == nil {
		entry = s.ProvinceByV1Code[code]
	}
	if entry == nil {
		entry = s.ProvinceByV1Name[normQuery(query)]
	}

	if entry == nil {
		return &provinceResolution{Found: false}
	}
	return &provinceResolution{
		Found: true,
		V2Province: &provinceRef{
			Code: entry.V2Code,
			Name: entry.V2Name,
		},
		V2ID: entry.V2ID,
	}
}

// resolveProvinceNewToOld looks up a v2 province and returns all v1 provinces
// that were merged into it.
func resolveProvinceNewToOld(s *loader.Store, query string) *provinceOldResolution {
	var entry *models.ProvinceMapEntry

	if code, err := strconv.Atoi(query); err == nil {
		entry = s.ProvinceByV2Code[code]
	}
	if entry == nil {
		entry = s.ProvinceByV2Name[normQuery(query)]
	}

	if entry == nil {
		return &provinceOldResolution{Found: false, V1Provinces: []provinceRef{}}
	}

	old := make([]provinceRef, 0, len(entry.V1Codes))
	for i, code := range entry.V1Codes {
		name := ""
		if i < len(entry.V1Names) {
			name = entry.V1Names[i]
		}
		old = append(old, provinceRef{Code: code, Name: name})
	}
	return &provinceOldResolution{
		Found: true,
		V2Province: &provinceRef{
			Code: entry.V2Code,
			Name: entry.V2Name,
		},
		V2ID:        entry.V2ID,
		V1Provinces: old,
	}
}

// resolveWardOldToNew looks up a v1 ward (by code or name) and returns the
// v2 ward it became.
func resolveWardOldToNew(s *loader.Store, query string) *wardNewResolution {
	var entry *models.WardMapEntry

	if code, err := strconv.Atoi(query); err == nil {
		entry = s.WardByV1Code[code]
	}
	if entry == nil {
		entry = s.WardByV1Name[normQuery(query)]
	}

	if entry == nil {
		return &wardNewResolution{Found: false, V1Wards: []wardRef{}}
	}

	// Collect the matched old wards (for transparency)
	v1 := make([]wardRef, 0, len(entry.V1Wards))
	for _, ow := range entry.V1Wards {
		v1 = append(v1, wardRef{
			Code:         ow.Code,
			Name:         ow.Name,
			DistrictCode: ow.DistrictCode,
			DistrictName: ow.DistrictName,
			ProvinceCode: ow.ProvinceCode,
			ProvinceName: ow.ProvinceName,
		})
	}

	return &wardNewResolution{
		Found: true,
		V2Ward: &wardRef{
			Code:         entry.V2Code,
			Name:         entry.V2Name,
			ProvinceCode: entry.V2ProvinceCode,
		},
		V1Wards: v1,
	}
}

// resolveWardNewToOld looks up a v2 ward (by code or name) and returns all
// the v1 wards it was formed from.
func resolveWardNewToOld(s *loader.Store, query string) *wardOldResolution {
	var entry *models.WardMapEntry

	if code, err := strconv.Atoi(query); err == nil {
		entry = s.WardByV2Code[code]
	}
	if entry == nil {
		entry = s.WardByV2Name[normQuery(query)]
	}

	if entry == nil {
		return &wardOldResolution{Found: false, V1Wards: []wardRef{}}
	}

	v1 := make([]wardRef, 0, len(entry.V1Wards))
	for _, ow := range entry.V1Wards {
		v1 = append(v1, wardRef{
			Code:         ow.Code,
			Name:         ow.Name,
			DistrictCode: ow.DistrictCode,
			DistrictName: ow.DistrictName,
			ProvinceCode: ow.ProvinceCode,
			ProvinceName: ow.ProvinceName,
		})
	}

	return &wardOldResolution{
		Found: true,
		V2Ward: &wardRef{
			Code:         entry.V2Code,
			Name:         entry.V2Name,
			ProvinceCode: entry.V2ProvinceCode,
		},
		V1Wards: v1,
	}
}

// normQuery applies the same normalisation as the loader index keys.
func normQuery(s string) string {
	s = strings.ToLower(s)
	for _, prefix := range []string{
		"thành phố ", "tỉnh ", "quận ", "huyện ", "thị xã ", "thị trấn ",
		"phường ", "xã ",
	} {
		s = strings.TrimPrefix(s, prefix)
	}
	return strings.TrimSpace(s)
}
