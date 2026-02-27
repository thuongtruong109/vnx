package vnx

import (
	"fmt"
	"strconv"
)

// V1ProvinceDetail is the response from [GetV1ProvinceDetail].
type V1ProvinceDetail struct {
	ID        string          `json:"id"`
	Province  *V1ProvinceInfo `json:"province"`
	Districts []District      `json:"districts"`
}

// ListV1Provinces returns the full list of pre-2025 (v1) provinces derived
// from map.json.
func ListV1Provinces(store *Store) []V1Province {
	return store.V1Provinces
}

// GetV1ProvinceDetail returns districts and wards for a single pre-2025 (v1)
// province. param may be either the numeric code (e.g. "70") or the slug id
// (e.g. "binhphuoc"). Returns an error when the province is not found.
func GetV1ProvinceDetail(store *Store, param string) (*V1ProvinceDetail, error) {
	slug := param
	if code, err := strconv.Atoi(param); err == nil {
		if s, ok := store.V1CodeToSlug[code]; ok {
			slug = s
		}
	}

	entry, ok := store.V1AddressBySlug[slug]
	if !ok {
		return nil, fmt.Errorf("v1 province not found: %s", param)
	}

	info := store.V1ProvincesBySlug[slug]
	districts := entry.Districts
	if districts == nil {
		districts = []District{}
	}

	return &V1ProvinceDetail{
		ID:        slug,
		Province:  info,
		Districts: districts,
	}, nil
}
