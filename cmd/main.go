package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"vnx-api/internal/handlers"
	"vnx-api/internal/loader"
)

func main() {
	// Resolve data directory relative to the executable
	dataDir := dataDirectory()

	log.Printf("Loading data from: %s", dataDir)
	store, err := loader.Load(dataDir)
	if err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}
	log.Printf("Loaded %d provinces, %d address entries", len(store.Provinces), len(store.Addresses))

	h := handlers.New(store)

	mux := http.NewServeMux()

	// Province routes
	mux.HandleFunc("/api/provinces", h.ListProvinces)
	mux.HandleFunc("/api/provinces/detail/", h.GetProvinceDetail)
	mux.HandleFunc("/api/provinces/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		// /api/provinces/{id}/districts/{code}/wards/{wcode}
		case strings.Contains(path, "/wards/") && strings.Contains(path, "/districts/"):
			h.GetWard(w, r)

		// /api/provinces/{id}/districts/{code}/wards
		case strings.HasSuffix(path, "/wards") && strings.Contains(path, "/districts/"):
			h.ListWards(w, r)

		// /api/provinces/{id}/districts/{code}
		case strings.Contains(path, "/districts/"):
			h.GetDistrict(w, r)

		// /api/provinces/{id}/districts
		case strings.HasSuffix(path, "/districts"):
			h.ListDistricts(w, r)

		// /api/provinces/{id}
		default:
			h.GetProvince(w, r)
		}
	})

	// Search
	mux.HandleFunc("/api/search", h.Search)

	// Regions
	mux.HandleFunc("/api/regions", h.ListRegions)

	// Address resolution (v1 ↔ v2)
	mux.HandleFunc("/api/resolve/old-to-new", h.ResolveOldToNew)
	mux.HandleFunc("/api/resolve/new-to-old", h.ResolveNewToOld)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("VNX API server listening on http://localhost%s", addr)
	log.Println("Available endpoints:")
	log.Println("  GET /health")
	log.Println("  GET /api/provinces")
	log.Println("  GET /api/provinces/{province_id}")
	log.Println("  GET /api/provinces/detail/{province_id}")
	log.Println("  GET /api/provinces/{province_id}/districts")
	log.Println("  GET /api/provinces/{province_id}/districts/{district_name}")
	log.Println("  GET /api/provinces/{province_id}/districts/{district_name}/wards")
	log.Println("  GET /api/provinces/{province_id}/districts/{district_name}/wards/{ward_name}")
	log.Println("  GET /api/regions")
	log.Println("  GET /api/search?q=<keyword>&type=province|district|ward")
	log.Println("  GET /api/resolve/old-to-new?province=<name|code>&district=<name>&ward=<name|code>")
	log.Println("  GET /api/resolve/new-to-old?province=<name|code>&ward=<name|code>")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// dataDirectory returns the path to the data directory.
// Priority: DATA_DIR env var → source tree (for `go run`).
// Set DATA_VERSION=v1 to use legacy data; default is v2 (post-2025).
func dataDirectory() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	version := os.Getenv("DATA_VERSION")
	if version == "" {
		version = "v2"
	}
	// When using `go run`, use the source file location
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(filename), "..", "data", version)
	}
	return "data/" + version
}
