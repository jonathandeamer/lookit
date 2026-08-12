package tui

import (
	"reflect"
	"slices"
	"testing"
)

func catalogFixture() []startEntry {
	return []startEntry{
		{target: "@tilde.team", kind: kindCommunity, note: "Small public access unix", source: sourceCatalog},
		{target: "@plan.cat", kind: kindCommunity, note: "Classic finger, polished for the present", source: sourceCatalog},
		{target: "quake@bbs.airandwave.net", kind: kindService, note: "Latest earthquakes", source: sourceCatalog},
	}
}

func TestBuildSectionsCatalogOnly(t *testing.T) {
	got := buildSections(catalogFixture(), bookmarkFile{})
	if len(got) != 2 {
		t.Fatalf("sections = %d (%+v), want 2", len(got), got)
	}
	if got[0].title != "COMMUNITIES" || len(got[0].entries) != 2 {
		t.Errorf("section 0 = %+v", got[0])
	}
	if got[1].title != "SERVICES" || len(got[1].entries) != 1 {
		t.Errorf("section 1 = %+v", got[1])
	}
}

func TestBuildSectionsAssignsStableIDs(t *testing.T) {
	got := buildSections(catalogFixture(), bookmarkFile{targets: []string{"@tilde.team"}})
	if got[0].id != sectionBookmarks || got[1].id != sectionCommunities || got[2].id != sectionServices {
		t.Fatalf("section IDs = [%v %v %v]", got[0].id, got[1].id, got[2].id)
	}
}

func TestBuildSectionsBookmarksComeFirstAndDedup(t *testing.T) {
	bm := bookmarkFile{targets: []string{"@tilde.team"}}
	got := buildSections(catalogFixture(), bm)
	if got[0].title != "BOOKMARKS" {
		t.Fatalf("section 0 title = %q, want BOOKMARKS", got[0].title)
	}
	if len(got[0].entries) != 1 || got[0].entries[0].target != "@tilde.team" {
		t.Fatalf("bookmarks section = %+v", got[0].entries)
	}
	// The note travels with the target even though the file stores none.
	if got[0].entries[0].note != "Small public access unix" {
		t.Errorf("note = %q, want the catalog's note", got[0].entries[0].note)
	}
	// And it is suppressed from COMMUNITIES rather than appearing twice.
	for _, e := range got[1].entries {
		if e.target == "@tilde.team" {
			t.Error("@tilde.team appears in both BOOKMARKS and COMMUNITIES")
		}
	}
}

func TestBuildSectionsBookmarkWithoutCatalogMatchHasNoDescription(t *testing.T) {
	bm := bookmarkFile{targets: []string{"weather:99501@bbs.airandwave.net"}}
	got := buildSections(catalogFixture(), bm)
	entry := got[0].entries[0]
	if entry.note != "" {
		t.Fatalf("note = %q, want blank for an unclassified bookmark", entry.note)
	}
	if entry.kind != kindUnknown {
		t.Fatalf("kind = %v, want no inferred classification", entry.kind)
	}
}

func TestBuildSectionsCatalogOff(t *testing.T) {
	bm := bookmarkFile{
		catalogHidden: true,
		targets:       []string{"@plan.cat"},
	}
	got := buildSections(catalogFixture(), bm)
	if len(got) != 1 || got[0].title != "BOOKMARKS" {
		t.Fatalf("sections = %+v, want BOOKMARKS only", got)
	}
	// A hidden catalog still supplies notes for matching bookmarks.
	if got[0].entries[0].note != "Classic finger, polished for the present" {
		t.Errorf("note = %q, want the catalog's note", got[0].entries[0].note)
	}
}

func TestBuildSectionsEmpty(t *testing.T) {
	if got := buildSections(nil, bookmarkFile{catalogHidden: true}); len(got) != 0 {
		t.Fatalf("sections = %+v, want none", got)
	}
}

func TestEntryHostAndToken(t *testing.T) {
	tests := []struct {
		target string
		host   string
		token  string
	}{
		{target: "@graph.no", host: "graph.no", token: ""},
		{target: "dict@bbs.airandwave.net", host: "bbs.airandwave.net", token: "dict"},
		{target: "wordsearch:today@bbs.airandwave.net", host: "bbs.airandwave.net", token: "wordsearch:today"},
		{target: "ring@thebackupbox.net", host: "thebackupbox.net", token: "ring"},
		{target: "1@happynetbox.com", host: "happynetbox.com", token: "1"},
	}
	for _, tt := range tests {
		if got := entryHost(tt.target); got != tt.host {
			t.Errorf("entryHost(%q) = %q, want %q", tt.target, got, tt.host)
		}
		if got := entryToken(tt.target); got != tt.token {
			t.Errorf("entryToken(%q) = %q, want %q", tt.target, got, tt.token)
		}
	}
}

func sectionTargets(t *testing.T, sections []startSection, id startSectionID) []string {
	t.Helper()
	for _, s := range sections {
		if s.id == id {
			targets := make([]string, 0, len(s.entries))
			for _, e := range s.entries {
				targets = append(targets, e.target)
			}
			return targets
		}
	}
	t.Fatalf("section %v not found", id)
	return nil
}

func TestCommunitiesSortAlphabeticallyByHost(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	want := []string{
		"@cosmic.voyage",
		"@happynetbox.com",
		"@plan.cat",
		"ring@thebackupbox.net",
		"@tilde.team",
		"@zaibatsu.circumlunar.space",
	}
	if got := sectionTargets(t, sections, sectionCommunities); !reflect.DeepEqual(got, want) {
		t.Fatalf("communities = %v, want %v", got, want)
	}
}

// A queried community sorts on its host, but only service rows participate in
// parent/child grouping — even if that host also has a root catalog entry.
func TestQueriedCommunitySortsByHostButStaysFlat(t *testing.T) {
	catalog := []startEntry{
		{target: "ring@thebackupbox.net", kind: kindCommunity, note: "Ring", source: sourceCatalog},
		{target: "@thebackupbox.net", kind: kindService, note: "Root", source: sourceCatalog},
	}
	sections := buildSections(catalog, bookmarkFile{})
	for _, section := range sections {
		if section.id != sectionCommunities {
			continue
		}
		if got := section.entries[0]; got.child || got.structural {
			t.Fatalf("queried community = %+v, want a plain sorted row", got)
		}
		return
	}
	t.Fatal("COMMUNITIES section not found")
}

func TestServicesGroupUnderHostRoots(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	want := []string{
		"@bbs.airandwave.net",
		"dict@bbs.airandwave.net",
		"quake@bbs.airandwave.net",
		"sudoku:easy@bbs.airandwave.net",
		"urban@bbs.airandwave.net",
		"weather@bbs.airandwave.net",
		"wordsearch:today@bbs.airandwave.net",
		"@flanigan.us",
		"calendar@flanigan.us",
		"@graph.no",
		"@happynetbox.com",
		"1@happynetbox.com",
		"bot@happynetbox.com",
		"browserversion@happynetbox.com",
		"originsfinger@happynetbox.com",
		"random@happynetbox.com",
		"@typed-hole.org",
		"cyoa@typed-hole.org",
		"smog@typed-hole.org",
		"textfile@typed-hole.org",
	}
	if got := sectionTargets(t, sections, sectionServices); !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %v, want %v", got, want)
	}
}

// @graph.no has a root and no children, so it is a plain row: no indent.
func TestRootWithoutChildrenIsNotAParent(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	for _, s := range sections {
		for _, e := range s.entries {
			if e.target == "@graph.no" && (e.child || e.structural) {
				t.Fatalf("@graph.no = %+v; want a plain row", e)
			}
		}
	}
}

// @happynetbox.com is a community listing AND the parent of its services, so
// the services copy is structural: a duplicate that exists only as structure.
func TestDualRoleHostAppearsInBothSections(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	communities := sectionTargets(t, sections, sectionCommunities)
	if !slices.Contains(communities, "@happynetbox.com") {
		t.Fatalf("communities = %v, want @happynetbox.com", communities)
	}
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if s.entries[10].target != "@happynetbox.com" || !s.entries[10].structural {
			t.Fatalf("services[10] = %+v, want a structural @happynetbox.com", s.entries[10])
		}
		if s.entries[11].target != "1@happynetbox.com" || !s.entries[11].child {
			t.Fatalf("services[11] = %+v, want a child row", s.entries[11])
		}
	}
}
