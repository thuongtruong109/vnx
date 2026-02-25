package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"vnx-api/internal/models"
)

// Store holds all loaded data in memory
type Store struct {
	Provinces []models.Province
	Addresses []models.AddressEntry

	// V1Provinces is the flat list of all pre-2025 (v1) provinces derived from
	// map.json. Each entry in map.json may carry multiple v1_codes/v1_names
	// (e.g. Hà Nội stayed as-is, but Huế absorbed Quảng Trị + TT-Huế).
	V1Provinces []models.V1Province

	// V1ProvincesBySlug holds all pre-2025 province metadata (from data/v1/province.json)
	// keyed by their slug id (e.g. "hanoi", "binhphuoc").
	V1ProvincesBySlug map[string]*models.V1ProvinceInfo

	// V1CodeToSlug maps a v1 numeric code -> slug id (e.g. 70 -> "binhphuoc")
	V1CodeToSlug map[int]string

	// V1AddressBySlug holds districts+wards for each v1 province, keyed by slug.
	V1AddressBySlug map[string]*models.AddressEntry

	// Index maps: province string id -> AddressEntry
	AddressByProvince map[string]*models.AddressEntry

	// Resolution map (data/map.json) — keyed for fast look-up
	// v2 ward code  -> WardMapEntry
	WardByV2Code map[int]*models.WardMapEntry
	// v1 ward code  -> WardMapEntry (the v2 ward it became)
	WardByV1Code map[int]*models.WardMapEntry
	// v2 province code -> ProvinceMapEntry
	ProvinceByV2Code map[int]*models.ProvinceMapEntry
	// v1 province code -> ProvinceMapEntry (the v2 province it became)
	ProvinceByV1Code map[int]*models.ProvinceMapEntry

	// normalised name -> ProvinceMapEntry (v2 province name lookup)
	ProvinceByV2Name map[string]*models.ProvinceMapEntry
	// normalised name -> ProvinceMapEntry (v1 province name lookup)
	ProvinceByV1Name map[string]*models.ProvinceMapEntry

	// normalised ward name (+ province code) -> WardMapEntry
	WardByV2Name map[string]*models.WardMapEntry
	// normalised old ward name -> WardMapEntry
	WardByV1Name map[string]*models.WardMapEntry
}

// Load reads province.json then loads each {province_id}.json file from dataDir.
// Province IDs are the string slugs defined in province.json (e.g. "hanoi", "tphcm").
// It also loads data/map.json (one level above dataDir) for address resolution.
func Load(dataDir string) (*Store, error) {
	store := &Store{
		AddressByProvince: make(map[string]*models.AddressEntry),
		V1ProvincesBySlug: make(map[string]*models.V1ProvinceInfo),
		V1CodeToSlug:      make(map[int]string),
		V1AddressBySlug:   make(map[string]*models.AddressEntry),
		WardByV2Code:      make(map[int]*models.WardMapEntry),
		WardByV1Code:      make(map[int]*models.WardMapEntry),
		ProvinceByV2Code:  make(map[int]*models.ProvinceMapEntry),
		ProvinceByV1Code:  make(map[int]*models.ProvinceMapEntry),
		ProvinceByV2Name:  make(map[string]*models.ProvinceMapEntry),
		ProvinceByV1Name:  make(map[string]*models.ProvinceMapEntry),
		WardByV2Name:      make(map[string]*models.WardMapEntry),
		WardByV1Name:      make(map[string]*models.WardMapEntry),
	}

	// 1. Load provinces
	provinceFile := fmt.Sprintf("%s/province.json", dataDir)
	pData, err := os.ReadFile(provinceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read province.json: %w", err)
	}
	if err := json.Unmarshal(pData, &store.Provinces); err != nil {
		return nil, fmt.Errorf("failed to parse province.json: %w", err)
	}

	// 2. For each province, load its individual {id}.json address file
	for _, p := range store.Provinces {
		addrFile := fmt.Sprintf("%s/%s.json", dataDir, p.ID)
		raw, err := os.ReadFile(addrFile)
		if err != nil {
			// Address file may not exist for every province — skip silently
			continue
		}

		// Each file is a JSON array of AddressEntry objects
		var entries []models.AddressEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse %s.json: %w", p.ID, err)
		}

		// Merge into global list and index by the province string id
		for i := range entries {
			entry := entries[i]
			// Override ProvinceID with the canonical string slug from province.json
			entry.ProvinceID = p.ID
			store.Addresses = append(store.Addresses, entry)
			last := &store.Addresses[len(store.Addresses)-1]
			store.AddressByProvince[p.ID] = last
		}
	}

	// 3. Load data/v1/ province list and per-province address files
	v1Dir := resolveV1Dir(dataDir)
	v1ProvinceFile := fmt.Sprintf("%s/province.json", v1Dir)
	if v1PData, err := os.ReadFile(v1ProvinceFile); err == nil {
		var v1Provs []models.V1ProvinceInfo
		if err := json.Unmarshal(v1PData, &v1Provs); err == nil {
			for i := range v1Provs {
				p := &v1Provs[i]
				store.V1ProvincesBySlug[p.ID] = p
				store.V1CodeToSlug[p.Code] = p.ID

				// Load per-province address file (districts + wards)
				addrFile := fmt.Sprintf("%s/%s.json", v1Dir, p.ID)
				if raw, err := os.ReadFile(addrFile); err == nil {
					var entries []models.AddressEntry
					if err := json.Unmarshal(raw, &entries); err == nil && len(entries) > 0 {
						entry := entries[0]
						entry.ProvinceID = p.ID
						store.V1AddressBySlug[p.ID] = &entry
					}
				}
			}
		}
	}

	// 4. Load data/map.json (located one level above versioned dataDir)
	mapFile := resolveMapFile(dataDir)
	if raw, err := os.ReadFile(mapFile); err == nil {
		var am models.AddressMap
		if err := json.Unmarshal(raw, &am); err != nil {
			return nil, fmt.Errorf("failed to parse map.json: %w", err)
		}

		// Index provinces
		seenV1Codes := make(map[int]bool)
		for i := range am.Provinces {
			pe := &am.Provinces[i]
			store.ProvinceByV2Code[pe.V2Code] = pe
			store.ProvinceByV2Name[normName(pe.V2Name)] = pe
			for j, vc := range pe.V1Codes {
				store.ProvinceByV1Code[vc] = pe
				if j < len(pe.V1Names) {
					store.ProvinceByV1Name[normName(pe.V1Names[j])] = pe
					if !seenV1Codes[vc] {
						seenV1Codes[vc] = true
						store.V1Provinces = append(store.V1Provinces, models.V1Province{
							Code: vc,
							Name: pe.V1Names[j],
						})
					}
				}
			}
		}

		// Index wards
		for i := range am.Wards {
			we := &am.Wards[i]
			store.WardByV2Code[we.V2Code] = we
			store.WardByV2Name[normName(we.V2Name)] = we
			for _, ow := range we.V1Wards {
				store.WardByV1Code[ow.Code] = we
				store.WardByV1Name[normName(ow.Name)] = we
			}
		}
	}

	return store, nil
}

// resolveV1Dir returns the path to data/v1/ directory.
// It lives at <repo-root>/data/v1/, one directory above the versioned dataDir (data/v2).
func resolveV1Dir(dataDir string) string {
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		candidate := filepath.Join(filepath.Dir(filename), "..", "..", "data", "v1")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	parent := filepath.Dir(dataDir)
	return filepath.Join(parent, "v1")
}

// resolveMapFile returns the path to data/map.json.
// It lives at <repo-root>/data/map.json, one directory above the versioned dataDir.
func resolveMapFile(dataDir string) string {
	// Try relative to source file (go run)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		candidate := filepath.Join(filepath.Dir(filename), "..", "..", "data", "map.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Fall back: go up one dir from dataDir (e.g. data/v2 -> data)
	parent := filepath.Dir(dataDir)
	return filepath.Join(parent, "map.json")
}

// normName lowercases and strips common Vietnamese administrative prefixes for
// flexible name matching (e.g. "Tỉnh Hà Nội" == "Hà Nội").
func normName(s string) string {
	s = strings.ToLower(s)
	for _, prefix := range []string{
		"thành phố ", "tỉnh ", "quận ", "huyện ", "thị xã ", "thị trấn ",
		"phường ", "xã ",
	} {
		s = strings.TrimPrefix(s, prefix)
	}
	return strings.TrimSpace(s)
}

