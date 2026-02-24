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

	// 3. Load data/map.json (located one level above versioned dataDir)
	mapFile := resolveMapFile(dataDir)
	if raw, err := os.ReadFile(mapFile); err == nil {
		var am models.AddressMap
		if err := json.Unmarshal(raw, &am); err != nil {
			return nil, fmt.Errorf("failed to parse map.json: %w", err)
		}

		// Index provinces
		for i := range am.Provinces {
			pe := &am.Provinces[i]
			store.ProvinceByV2Code[pe.V2Code] = pe
			store.ProvinceByV2Name[normName(pe.V2Name)] = pe
			for j, vc := range pe.V1Codes {
				store.ProvinceByV1Code[vc] = pe
				if j < len(pe.V1Names) {
					store.ProvinceByV1Name[normName(pe.V1Names[j])] = pe
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

