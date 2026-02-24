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

	"github.com/jedib0t/go-pretty/v6/table"
)

func main() {
	// Resolve data directory relative to the executable
	dataDir := dataDirectory()

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

	// Legacy (v1) province list – sourced from map.json
	mux.HandleFunc("/api/v1/provinces", h.ListV1Provinces)

	// Address resolution (v1 ↔ v2)
	mux.HandleFunc("/api/resolve/old-to-new", h.ResolveOldToNew)
	mux.HandleFunc("/api/resolve/new-to-old", h.ResolveNewToOld)

	// Demo page – serve static/index.html
	staticDir := staticDirectory()
	mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
	// Serve other static assets (css, js, etc.) under /static/
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

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
	printEndpointsTable()

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

// staticDirectory returns the path to the static assets directory.
// Priority: STATIC_DIR env var → source tree (for `go run`).
func staticDirectory() string {
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		return dir
	}
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(filename), "..", "static")
	}
	return "static"
}

// printEndpointsTable prints a colored table of available endpoints.
func printEndpointsTable() {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"METHOD", "PATH", "DESCRIPTION"})
	t.AppendRows([]table.Row{
		{"GET", "/health", "Health check"},
		{"GET", "/api/provinces", "List provinces"},
		{"GET", "/api/provinces/{province_id}", "Get province"},
		{"GET", "/api/provinces/detail/{province_id}", "Province detail (with districts/wards)"},
		{"GET", "/api/provinces/{province_id}/districts", "List districts"},
		{"GET", "/api/provinces/{province_id}/districts/{district_name}", "Get district"},
		{"GET", "/api/provinces/{province_id}/districts/{district_name}/wards", "List wards"},
		{"GET", "/api/provinces/{province_id}/districts/{district_name}/wards/{ward_name}", "Get ward"},
		{"GET", "/api/regions", "Provinces grouped by region"},
		{"GET", "/api/v1/provinces", "List all pre-2025 (v1) provinces"},
		{"GET", "/api/search?q=<keyword>&type=province|district|ward", "Search by name"},
		{"GET", "/api/resolve/old-to-new?province=...&ward=...", "Resolve v1 → v2"},
		{"GET", "/api/resolve/new-to-old?province=...&ward=...", "Resolve v2 → v1"},
		{"GET", "/demo", "Demo page – address converter UI"},
	})
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = true
	t.Render()
}
