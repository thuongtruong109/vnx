package models

// Province represents a Vietnamese province/city
type Province struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	DivisionType string   `json:"division_type"`
	PhoneCode    int      `json:"phone_code"`
	LicensePlate string   `json:"license_plate"`
	Type         string   `json:"type"`
	Region       string   `json:"region"`
	AreaKm2      float64  `json:"area_km2"`
	Population   int      `json:"population"`
	Boundaries   []string `json:"boundaries"`
}

// WardDetails contains streets and villages/hamlets of a ward
type WardDetails struct {
	Streets         []string `json:"streets"`
	VillagesHamlets []string `json:"villages_hamlets"`
}

// Ward represents a ward/commune/town within a district
type Ward struct {
	Name         string      `json:"name"`
	DivisionType string      `json:"division_type"`
	Details      WardDetails `json:"details"`
}

// District represents a district within a province
type District struct {
	Name         string `json:"name"`
	DivisionType string `json:"division_type"`
	Wards        []Ward `json:"wards"`
}

// AddressEntry links a province_id with its districts
type AddressEntry struct {
	ProvinceID string     `json:"province_id"`
	Districts  []District `json:"districts"`
}

// ProvinceDetail combines full province info with its address data
type ProvinceDetail struct {
	Province
	Districts []District `json:"districts"`
}
