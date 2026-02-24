package loader

import (
	"encoding/json"
	"fmt"
	"os"

	"vnx-api/internal/models"
)

// Store holds all loaded data in memory
type Store struct {
	Provinces []models.Province
	Addresses []models.AddressEntry

	// Index maps: province string id -> AddressEntry
	AddressByProvince map[string]*models.AddressEntry
}

// Load reads province.json then loads each {province_id}.json file from dataDir.
// Province IDs are the string slugs defined in province.json (e.g. "hanoi", "tphcm").
func Load(dataDir string) (*Store, error) {
	store := &Store{
		AddressByProvince: make(map[string]*models.AddressEntry),
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

	return store, nil
}
