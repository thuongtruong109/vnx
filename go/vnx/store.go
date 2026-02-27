package vnx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Store holds all loaded data in memory.
// Obtain one by calling [Load].
type Store struct {
	Provinces []Province
	Addresses []AddressEntry

	// V1Provinces is the flat list of all pre-2025 (v1) provinces derived from map.json.
	V1Provinces []V1Province

	// V1ProvincesBySlug holds pre-2025 province metadata keyed by slug id.
	V1ProvincesBySlug map[string]*V1ProvinceInfo

	// V1CodeToSlug maps a v1 numeric code → slug id (e.g. 70 → "binhphuoc").
	V1CodeToSlug map[int]string

	// V1AddressBySlug holds districts+wards for each v1 province, keyed by slug.
	V1AddressBySlug map[string]*AddressEntry

	// AddressByProvince indexes province string id → AddressEntry.
	AddressByProvince map[string]*AddressEntry

	// WardByV2Code — v2 ward code → WardMapEntry
	WardByV2Code map[int]*WardMapEntry
	// WardByV1Code — v1 ward code → WardMapEntry (the v2 ward it became)
	WardByV1Code map[int]*WardMapEntry
	// ProvinceByV2Code — v2 province code → ProvinceMapEntry
	ProvinceByV2Code map[int]*ProvinceMapEntry
	// ProvinceByV1Code — v1 province code → ProvinceMapEntry (the v2 province it became)
	ProvinceByV1Code map[int]*ProvinceMapEntry

	// ProvinceByV2Name — normalised v2 province name → ProvinceMapEntry
	ProvinceByV2Name map[string]*ProvinceMapEntry
	// ProvinceByV1Name — normalised v1 province name → ProvinceMapEntry
	ProvinceByV1Name map[string]*ProvinceMapEntry

	// WardByV2Name — normalised v2 ward name → WardMapEntry
	WardByV2Name map[string]*WardMapEntry
	// WardByV1Name — normalised v1 ward name → WardMapEntry
	WardByV1Name map[string]*WardMapEntry
}

// Load reads province.json then loads each {province_id}.json file from dataDir.
// It also loads data/map.json (one level above dataDir) for address resolution.
func Load(dataDir string) (*Store, error) {
	store := &Store{
		AddressByProvince: make(map[string]*AddressEntry),
		V1ProvincesBySlug: make(map[string]*V1ProvinceInfo),
		V1CodeToSlug:      make(map[int]string),
		V1AddressBySlug:   make(map[string]*AddressEntry),
		WardByV2Code:      make(map[int]*WardMapEntry),
		WardByV1Code:      make(map[int]*WardMapEntry),
		ProvinceByV2Code:  make(map[int]*ProvinceMapEntry),
		ProvinceByV1Code:  make(map[int]*ProvinceMapEntry),
		ProvinceByV2Name:  make(map[string]*ProvinceMapEntry),
		ProvinceByV1Name:  make(map[string]*ProvinceMapEntry),
		WardByV2Name:      make(map[string]*WardMapEntry),
		WardByV1Name:      make(map[string]*WardMapEntry),
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
			continue // address file may not exist for every province
		}

		var entries []AddressEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse %s.json: %w", p.ID, err)
		}

		for i := range entries {
			entry := entries[i]
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
		var v1Provs []V1ProvinceInfo
		if err := json.Unmarshal(v1PData, &v1Provs); err == nil {
			for i := range v1Provs {
				p := &v1Provs[i]
				store.V1ProvincesBySlug[p.ID] = p
				store.V1CodeToSlug[p.Code] = p.ID

				addrFile := fmt.Sprintf("%s/%s.json", v1Dir, p.ID)
				if raw, err := os.ReadFile(addrFile); err == nil {
					var entries []AddressEntry
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
		var am AddressMap
		if err := json.Unmarshal(raw, &am); err != nil {
			return nil, fmt.Errorf("failed to parse map.json: %w", err)
		}

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
						store.V1Provinces = append(store.V1Provinces, V1Province{
							Code: vc,
							Name: pe.V1Names[j],
						})
					}
				}
			}
		}

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
func resolveMapFile(dataDir string) string {
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		candidate := filepath.Join(filepath.Dir(filename), "..", "..", "data", "map.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
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
