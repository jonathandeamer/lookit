package tui

import _ "embed"

// catalogData is lookit's curated starting list, compiled in so it can be
// improved for every user on each release. Seeding these into the user's file
// instead would freeze them forever at the version first run.
//
//go:embed catalog.txt
var catalogData []byte

// loadCatalog parses the embedded catalog. Bad lines are impossible in a
// released binary — TestCatalogIsWellFormed fails the build gate first — so
// any that appear at runtime are skipped silently rather than shown to a user
// who cannot fix them.
func loadCatalog() []startEntry {
	entries, _ := parseCatalogData(catalogData)
	return entries
}
