// Package vnx provides data types and query functions for Vietnamese
// administrative divisions (tỉnh/thành phố, quận/huyện, phường/xã).
//
// Use [Load] to populate a [Store] from the bundled JSON data, then call the
// various query helpers (e.g. [GetProvinces], [GetProvince], [Search], …) to
// access the data without having to touch the raw JSON files yourself.
package vnx

// AdminStatus represents the lifecycle state of an administrative unit.
//
//   - "active"  — currently in use
//   - "merged"  — absorbed into another unit (see MergedInto)
//   - "split"   — divided into multiple units (see MergedInto for successor)
//   - "renamed" — still exists but under a new name
type AdminStatus string

const (
	StatusActive  AdminStatus = "active"
	StatusMerged  AdminStatus = "merged"
	StatusSplit   AdminStatus = "split"
	StatusRenamed AdminStatus = "renamed"
)

// IsActive returns true when an entity should be shown in normal (active-only) queries.
// Entities with no Status set are treated as active (backward-compatible).
func IsActive(s AdminStatus) bool {
	return s == "" || s == StatusActive
}

// Province represents a Vietnamese province/city.
type Province struct {
	ID           string   `json:"id"`
	Code         int      `json:"code,omitempty"`
	Name         string   `json:"name"`
	DivisionType string   `json:"division_type"`
	PhoneCode    int      `json:"phone_code"`
	LicensePlate string   `json:"license_plate"`
	Type         string   `json:"type"`
	Region       string   `json:"region"`
	AreaKm2      float64  `json:"area_km2"`
	Population   int      `json:"population"`
	Boundaries   []string `json:"boundaries"`

	Status        AdminStatus `json:"status,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	EndDate       string      `json:"end_date,omitempty"`
	MergedFrom    []string    `json:"merged_from,omitempty"`
	MergedInto    string      `json:"merged_into,omitempty"`
}

// WardDetails contains streets and villages/hamlets of a ward.
type WardDetails struct {
	Streets         []string `json:"streets"`
	VillagesHamlets []string `json:"villages_hamlets"`
}

// Ward represents a ward/commune/town within a district.
type Ward struct {
	Name         string      `json:"name"`
	DivisionType string      `json:"division_type"`
	Code         int         `json:"code,omitempty"`
	Details      WardDetails `json:"details"`

	Status        AdminStatus `json:"status,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	EndDate       string      `json:"end_date,omitempty"`
	MergedFrom    []string    `json:"merged_from,omitempty"`
	MergedInto    string      `json:"merged_into,omitempty"`
}

// District represents a district within a province.
type District struct {
	Name         string `json:"name"`
	DivisionType string `json:"division_type"`
	Wards        []Ward `json:"wards"`

	Status        AdminStatus `json:"status,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	EndDate       string      `json:"end_date,omitempty"`
	MergedFrom    []string    `json:"merged_from,omitempty"`
	MergedInto    string      `json:"merged_into,omitempty"`
}

// AddressEntry links a province_id with its districts.
type AddressEntry struct {
	ProvinceID string     `json:"province_id"`
	Districts  []District `json:"districts"`
}

// DistrictNoWards is a district summary without the wards list (deep=2).
type DistrictNoWards struct {
	Name          string      `json:"name"`
	DivisionType  string      `json:"division_type"`
	Status        AdminStatus `json:"status,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	EndDate       string      `json:"end_date,omitempty"`
	MergedFrom    []string    `json:"merged_from,omitempty"`
	MergedInto    string      `json:"merged_into,omitempty"`
}

// WardNoDetails is a ward without street/village details (deep=3).
type WardNoDetails struct {
	Name          string      `json:"name"`
	DivisionType  string      `json:"division_type"`
	Code          int         `json:"code,omitempty"`
	Status        AdminStatus `json:"status,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	EndDate       string      `json:"end_date,omitempty"`
	MergedFrom    []string    `json:"merged_from,omitempty"`
	MergedInto    string      `json:"merged_into,omitempty"`
}

// DistrictWithWardsNoDetails is a district with wards but without ward details (deep=3).
type DistrictWithWardsNoDetails struct {
	Name          string         `json:"name"`
	DivisionType  string         `json:"division_type"`
	Wards         []WardNoDetails `json:"wards"`
	Status        AdminStatus    `json:"status,omitempty"`
	EffectiveDate string         `json:"effective_date,omitempty"`
	EndDate       string         `json:"end_date,omitempty"`
	MergedFrom    []string       `json:"merged_from,omitempty"`
	MergedInto    string         `json:"merged_into,omitempty"`
}

// ---------------------------------------------------------------------------
// Address resolution map (data/map.json)
// ---------------------------------------------------------------------------

// OldWardRef is a reference to a pre-2025 ward (v1).
type OldWardRef struct {
	Code         int    `json:"code"`
	Name         string `json:"name"`
	DistrictCode int    `json:"district_code"`
	DistrictName string `json:"district_name"`
	ProvinceCode int    `json:"province_code"`
	ProvinceName string `json:"province_name"`
}

// WardMapEntry maps one v2 ward → all v1 wards it was formed from.
type WardMapEntry struct {
	V2Code         int          `json:"v2_code"`
	V2Name         string       `json:"v2_name"`
	V2ProvinceCode int          `json:"v2_province_code"`
	V1Wards        []OldWardRef `json:"v1_wards"`
}

// ProvinceMapEntry maps one v2 province → all v1 provinces it absorbed.
type ProvinceMapEntry struct {
	V2Code  int      `json:"v2_code"`
	V2ID    string   `json:"v2_id"`
	V2Name  string   `json:"v2_name"`
	V1Codes []int    `json:"v1_codes"`
	V1Names []string `json:"v1_names"`
}

// AddressMap is the top-level structure of data/map.json.
type AddressMap struct {
	Provinces []ProvinceMapEntry `json:"provinces"`
	Wards     []WardMapEntry     `json:"wards"`
}

// V1Province is a single pre-2025 province entry derived from map.json.
type V1Province struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

// V1ProvinceInfo is a pre-2025 province with full metadata, loaded from data/v1/province.json.
type V1ProvinceInfo struct {
	ID           string      `json:"id"`
	Code         int         `json:"code"`
	Name         string      `json:"name"`
	DivisionType string      `json:"division_type"`
	Status       AdminStatus `json:"status,omitempty"`
	MergedInto   string      `json:"merged_into,omitempty"`
}
