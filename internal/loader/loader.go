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

	// Index maps: province_id -> AddressEntry
	AddressByProvince map[string]*models.AddressEntry
}

// Load reads and parses both JSON data files from the given data directory
func Load(dataDir string) (*Store, error) {
	store := &Store{
		AddressByProvince: make(map[string]*models.AddressEntry),
	}

	// Load provinces
	provinceFile := fmt.Sprintf("%s/province.json", dataDir)
	pData, err := os.ReadFile(provinceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read province.json: %w", err)
	}
	if err := json.Unmarshal(pData, &store.Provinces); err != nil {
		return nil, fmt.Errorf("failed to parse province.json: %w", err)
	}

	// Load address
	addressFile := fmt.Sprintf("%s/address.json", dataDir)
	aData, err := os.ReadFile(addressFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read address.json: %w", err)
	}
	if err := json.Unmarshal(aData, &store.Addresses); err != nil {
		return nil, fmt.Errorf("failed to parse address.json: %w", err)
	}

	// Build index
	for i := range store.Addresses {
		entry := &store.Addresses[i]
		store.AddressByProvince[entry.ProvinceID] = entry
	}

	return store, nil
}
