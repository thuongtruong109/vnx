package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"vnx-api/internal/loader"
	"vnx-api/internal/models"
)

// Handler holds the data store and implements all HTTP endpoints
type Handler struct {
	store *loader.Store
}

// New creates a new Handler with the given store
func New(store *loader.Store) *Handler {
	return &Handler{store: store}
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseDeep reads the "deep" query parameter (1–4) and returns the integer value.
// Defaults to 1 when the parameter is absent or invalid.
//
//   deep=1  – province only
//   deep=2  – province + districts (no wards)
//   deep=3  – province + districts + wards (no street/village details)
//   deep=4  – province + districts + wards + street/village details
func parseDeep(r *http.Request) int {
	d, err := strconv.Atoi(r.URL.Query().Get("deep"))
	if err != nil || d < 1 || d > 4 {
		return 1
	}
	return d
}

// provinceAtDepth builds a response object for the given province at the
// requested depth level. includeInactive controls whether non-active
// districts/wards are included at deeper levels.
func (h *Handler) provinceAtDepth(p models.Province, depth int, includeInactive bool) any {
	if depth == 1 {
		return p
	}

	entry := h.store.AddressByProvince[p.ID]

	if depth == 2 {
		type resp struct {
			models.Province
			Districts []models.DistrictNoWards `json:"districts"`
		}
		r := resp{Province: p, Districts: []models.DistrictNoWards{}}
		if entry != nil {
			for _, d := range entry.Districts {
				if !includeInactive && !models.IsActive(d.Status) {
					continue
				}
				r.Districts = append(r.Districts, models.DistrictNoWards{
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
			models.Province
			Districts []models.DistrictWithWardsNoDetails `json:"districts"`
		}
		r := resp{Province: p, Districts: []models.DistrictWithWardsNoDetails{}}
		if entry != nil {
			for _, d := range entry.Districts {
				if !includeInactive && !models.IsActive(d.Status) {
					continue
				}
				wd := models.DistrictWithWardsNoDetails{
					Name:          d.Name,
					DivisionType:  d.DivisionType,
					Status:        d.Status,
					EffectiveDate: d.EffectiveDate,
					EndDate:       d.EndDate,
					MergedFrom:    d.MergedFrom,
					MergedInto:    d.MergedInto,
					Wards:         []models.WardNoDetails{},
				}
				for _, w := range d.Wards {
					if !includeInactive && !models.IsActive(w.Status) {
						continue
					}
					wd.Wards = append(wd.Wards, models.WardNoDetails{
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

	// depth == 4: full detail (province + districts + wards + street/village details)
	type resp struct {
		models.Province
		Districts []models.District `json:"districts"`
	}
	r := resp{Province: p, Districts: []models.District{}}
	if entry != nil {
		for _, d := range entry.Districts {
			if !includeInactive && !models.IsActive(d.Status) {
				continue
			}
			if includeInactive {
				r.Districts = append(r.Districts, d)
			} else {
				filtered := models.District{
					Name:          d.Name,
					DivisionType:  d.DivisionType,
					Status:        d.Status,
					EffectiveDate: d.EffectiveDate,
					EndDate:       d.EndDate,
					MergedFrom:    d.MergedFrom,
					MergedInto:    d.MergedInto,
					Wards:         []models.Ward{},
				}
				for _, w := range d.Wards {
					if models.IsActive(w.Status) {
						filtered.Wards = append(filtered.Wards, w)
					}
				}
				r.Districts = append(r.Districts, filtered)
			}
		}
	}
	return r
}

// -----------------------------------------------------------------
// GET /api/provinces?include_inactive=true&deep=1
// Returns list of provinces.
//   deep=1 (default) – province fields only
//   deep=2           – province + districts (no wards)
//   deep=3           – province + districts + wards (no details)
//   deep=4           – province + districts + wards + street/village details
// Active-only by default; pass include_inactive=true to include
// merged/split/renamed units.
// -----------------------------------------------------------------
func (h *Handler) ListProvinces(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	deep := parseDeep(r)

	results := make([]any, 0, len(h.store.Provinces))
	for _, p := range h.store.Provinces {
		if !includeInactive && !models.IsActive(p.Status) {
			continue
		}
		results = append(results, h.provinceAtDepth(p, deep, includeInactive))
	}
	writeJSON(w, http.StatusOK, results)
}

// -----------------------------------------------------------------
// GET /api/provinces/{province_id}?deep=1
// Returns a single province by id.
//   deep=1 (default) – province fields only
//   deep=2           – province + districts (no wards)
//   deep=3           – province + districts + wards (no details)
//   deep=4           – province + districts + wards + street/village details
// -----------------------------------------------------------------
func (h *Handler) GetProvince(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path, "/api/provinces/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing province id")
		return
	}

	deep := parseDeep(r)
	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	for _, p := range h.store.Provinces {
		if p.ID == id {
			writeJSON(w, http.StatusOK, h.provinceAtDepth(p, deep, includeInactive))
			return
		}
	}
	writeError(w, http.StatusNotFound, "province not found")
}

// -----------------------------------------------------------------
// GET /api/provinces/detail/{province_id}?deep=4
// Returns full province info merged with all its districts & wards.
// Supports the same deep=1..4 levels as /api/provinces/{id}.
// Defaults to deep=4 (full detail) if not specified.
// -----------------------------------------------------------------
func (h *Handler) GetProvinceDetail(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path, "/api/provinces/detail/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing province id")
		return
	}

	var found *models.Province
	for i := range h.store.Provinces {
		if h.store.Provinces[i].ID == id {
			found = &h.store.Provinces[i]
			break
		}
	}
	if found == nil {
		writeError(w, http.StatusNotFound, "province not found")
		return
	}

	// Default to deep=4 for backward-compatibility with this endpoint
	deep := 4
	if raw := r.URL.Query().Get("deep"); raw != "" {
		if d, err := strconv.Atoi(raw); err == nil && d >= 1 && d <= 4 {
			deep = d
		}
	}
	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	writeJSON(w, http.StatusOK, h.provinceAtDepth(*found, deep, includeInactive))
}

// -----------------------------------------------------------------
// GET /api/provinces/{province_id}/districts?include_inactive=true
// Returns all districts of a province. Active-only by default.
// -----------------------------------------------------------------
func (h *Handler) ListDistricts(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path, "/api/provinces/")
	id = strings.TrimSuffix(id, "/districts")

	entry, ok := h.store.AddressByProvince[id]
	if !ok {
		writeError(w, http.StatusNotFound, "province address data not found")
		return
	}

	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	type districtSummary struct {
		Name         string            `json:"name"`
		DivisionType string            `json:"division_type"`
		WardCount    int               `json:"ward_count"`
		Status       models.AdminStatus `json:"status,omitempty"`
		MergedInto   string            `json:"merged_into,omitempty"`
		MergedFrom   []string          `json:"merged_from,omitempty"`
		EffectiveDate string           `json:"effective_date,omitempty"`
		EndDate      string            `json:"end_date,omitempty"`
	}
	result := make([]districtSummary, 0, len(entry.Districts))
	for _, d := range entry.Districts {
		if !includeInactive && !models.IsActive(d.Status) {
			continue
		}
		result = append(result, districtSummary{
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
	writeJSON(w, http.StatusOK, result)
}

// -----------------------------------------------------------------
// GET /api/provinces/{province_id}/districts/{district_name}
// Returns a single district with all its wards
// -----------------------------------------------------------------
func (h *Handler) GetDistrict(w http.ResponseWriter, r *http.Request) {
	// path: /api/provinces/{id}/districts/{name}
	rest := pathParam(r.URL.Path, "/api/provinces/")
	parts := strings.SplitN(rest, "/districts/", 2)
	if len(parts) != 2 || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	provinceID := parts[0]
	districtName := parts[1]

	entry, ok := h.store.AddressByProvince[provinceID]
	if !ok {
		writeError(w, http.StatusNotFound, "province address data not found")
		return
	}

	for _, d := range entry.Districts {
		if d.Name == districtName {
			writeJSON(w, http.StatusOK, d)
			return
		}
	}
	writeError(w, http.StatusNotFound, "district not found")
}

// -----------------------------------------------------------------
// GET /api/provinces/{province_id}/districts/{district_name}/wards?include_inactive=true
// Returns all wards of a district. Active-only by default.
// -----------------------------------------------------------------
func (h *Handler) ListWards(w http.ResponseWriter, r *http.Request) {
	rest := pathParam(r.URL.Path, "/api/provinces/")
	// strip trailing /wards
	rest = strings.TrimSuffix(rest, "/wards")
	parts := strings.SplitN(rest, "/districts/", 2)
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	provinceID := parts[0]
	districtName := parts[1]

	entry, ok := h.store.AddressByProvince[provinceID]
	if !ok {
		writeError(w, http.StatusNotFound, "province address data not found")
		return
	}

	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	for _, d := range entry.Districts {
		if d.Name == districtName {
			if includeInactive {
				writeJSON(w, http.StatusOK, d.Wards)
				return
			}
			active := make([]models.Ward, 0, len(d.Wards))
			for _, w := range d.Wards {
				if models.IsActive(w.Status) {
					active = append(active, w)
				}
			}
			writeJSON(w, http.StatusOK, active)
			return
		}
	}
	writeError(w, http.StatusNotFound, "district not found")
}

// -----------------------------------------------------------------
// GET /api/provinces/{province_id}/districts/{district_name}/wards/{ward_name}
// Returns a single ward with street/village details
// -----------------------------------------------------------------
func (h *Handler) GetWard(w http.ResponseWriter, r *http.Request) {
	rest := pathParam(r.URL.Path, "/api/provinces/")
	districtParts := strings.SplitN(rest, "/districts/", 2)
	if len(districtParts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	provinceID := districtParts[0]
	wardParts := strings.SplitN(districtParts[1], "/wards/", 2)
	if len(wardParts) != 2 || wardParts[1] == "" {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	districtName := wardParts[0]
	wardName := wardParts[1]

	entry, ok := h.store.AddressByProvince[provinceID]
	if !ok {
		writeError(w, http.StatusNotFound, "province address data not found")
		return
	}

	for _, d := range entry.Districts {
		if d.Name == districtName {
			for _, ward := range d.Wards {
				if ward.Name == wardName {
					writeJSON(w, http.StatusOK, ward)
					return
				}
			}
			writeError(w, http.StatusNotFound, "ward not found")
			return
		}
	}
	writeError(w, http.StatusNotFound, "district not found")
}

// -----------------------------------------------------------------
// GET /api/search?q=...&type=province|district|ward&include_inactive=true
// Search by name across all entities. Active-only by default.
// -----------------------------------------------------------------
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	typ := r.URL.Query().Get("type")
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	if q == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	type result struct {
		Type         string            `json:"type"`
		ProvinceID   string            `json:"province_id,omitempty"`
		Name         string            `json:"name"`
		DivisionType string            `json:"division_type"`
		Status       models.AdminStatus `json:"status,omitempty"`
		MergedInto   string            `json:"merged_into,omitempty"`
	}

	var results []result

	if typ == "" || typ == "province" {
		for _, p := range h.store.Provinces {
			if !includeInactive && !models.IsActive(p.Status) {
				continue
			}
			if strings.Contains(strings.ToLower(p.Name), q) {
				results = append(results, result{
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
		for _, entry := range h.store.Addresses {
			for _, d := range entry.Districts {
				if (typ == "" || typ == "district") && strings.Contains(strings.ToLower(d.Name), q) {
					if includeInactive || models.IsActive(d.Status) {
						results = append(results, result{
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
							if includeInactive || models.IsActive(ward.Status) {
								results = append(results, result{
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
		results = []result{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   q,
		"total":   len(results),
		"results": results,
	})
}

// -----------------------------------------------------------------
// GET /api/regions
// Returns provinces grouped by region
// -----------------------------------------------------------------
func (h *Handler) ListRegions(w http.ResponseWriter, r *http.Request) {
	grouped := make(map[string][]models.Province)
	for _, p := range h.store.Provinces {
		grouped[p.Region] = append(grouped[p.Region], p)
	}
	writeJSON(w, http.StatusOK, grouped)
}

// -----------------------------------------------------------------
// GET /api/v1/provinces
// Returns the full list of pre-2025 (v1) provinces extracted from map.json.
// -----------------------------------------------------------------
func (h *Handler) ListV1Provinces(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.V1Provinces)
}

// -----------------------------------------------------------------
// GET /api/v1/provinces/{code_or_slug}
// Returns districts and wards for a single pre-2025 (v1) province.
// Accepts either the numeric code (e.g. 70) or slug id (e.g. binhphuoc).
// -----------------------------------------------------------------
func (h *Handler) GetV1ProvinceDetail(w http.ResponseWriter, r *http.Request) {
	param := pathParam(r.URL.Path, "/api/v1/provinces/")
	if param == "" {
		writeError(w, http.StatusBadRequest, "missing province code or slug")
		return
	}

	// Resolve param to slug: try numeric code first, then treat as slug directly
	slug := param
	if code, err := strconv.Atoi(param); err == nil {
		if s, ok := h.store.V1CodeToSlug[code]; ok {
			slug = s
		}
	}

	entry, ok := h.store.V1AddressBySlug[slug]
	if !ok {
		writeError(w, http.StatusNotFound, "v1 province not found: "+param)
		return
	}
	info := h.store.V1ProvincesBySlug[slug]

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        slug,
		"province":  info,
		"districts": entry.Districts,
	})
}

// pathParam extracts the URL segment after prefix
func pathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
