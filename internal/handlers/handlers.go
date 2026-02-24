package handlers

import (
	"encoding/json"
	"net/http"
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

// -----------------------------------------------------------------
// GET /api/provinces?include_inactive=true
// Returns list of provinces. Active-only by default; pass
// include_inactive=true to include merged/split/renamed units.
// -----------------------------------------------------------------
func (h *Handler) ListProvinces(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	if includeInactive {
		writeJSON(w, http.StatusOK, h.store.Provinces)
		return
	}
	active := make([]models.Province, 0, len(h.store.Provinces))
	for _, p := range h.store.Provinces {
		if models.IsActive(p.Status) {
			active = append(active, p)
		}
	}
	writeJSON(w, http.StatusOK, active)
}

// -----------------------------------------------------------------
// GET /api/provinces/{province_id}
// Returns a single province by id
// -----------------------------------------------------------------
func (h *Handler) GetProvince(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path, "/api/provinces/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing province id")
		return
	}

	for _, p := range h.store.Provinces {
		if p.ID == id {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	writeError(w, http.StatusNotFound, "province not found")
}

// -----------------------------------------------------------------
// GET /api/provinces/detail/{province_id}
// Returns full province info merged with all its districts & wards
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

	entry, ok := h.store.AddressByProvince[id]
	var districts []models.District
	if ok {
		districts = entry.Districts
	} else {
		districts = []models.District{}
	}

	writeJSON(w, http.StatusOK, models.ProvinceDetail{
		Province:  *found,
		Districts: districts,
	})
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

// pathParam extracts the URL segment after prefix
func pathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
