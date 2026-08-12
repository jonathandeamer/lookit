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

// A bookmarked target that is also a service child in the catalog (grouped
// under its host root everywhere else) must not carry child/lastChild into
// BOOKMARKS: bm.targets never passes through groupByHost, the only place
// those flags are ever set, so a bookmark row is always a listing — full
// target, full note, no connector — never a group member. smog@typed-hole.org
// is a real catalog service child, so this exercises the actual assembly
// path rather than a synthetic fixture.
func TestBuildSectionsBookmarkedServiceChildIsNotAChildRow(t *testing.T) {
	bm := bookmarkFile{targets: []string{"smog@typed-hole.org"}}
	got := buildSections(loadCatalog(), bm)
	if len(got) == 0 || got[0].id != sectionBookmarks {
		t.Fatalf("sections = %+v, want a BOOKMARKS section first", got)
	}
	for _, e := range got[0].entries {
		if e.child || e.lastChild {
			t.Errorf("%s has child=%v lastChild=%v; a BOOKMARKS row is a listing, not a group member", e.target, e.child, e.lastChild)
		}
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
		"bonsai@flanigan.us",
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
	found := false
	for _, s := range sections {
		for _, e := range s.entries {
			if e.target != "@graph.no" {
				continue
			}
			found = true
			if e.child || e.structural {
				t.Fatalf("@graph.no = %+v; want a plain row", e)
			}
		}
	}
	if !found {
		t.Fatal("@graph.no not found in any section")
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
	foundServices := false
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		foundServices = true
		// Located by target, not by index: this test is about the dual role,
		// and adding an unrelated catalog entry earlier in the ordering must
		// not be able to break it.
		parent := -1
		for i, e := range s.entries {
			if e.target == "@happynetbox.com" {
				parent = i
				break
			}
		}
		if parent < 0 {
			t.Fatalf("services = %v, want a @happynetbox.com parent row", sectionTargets(t, sections, sectionServices))
		}
		if !s.entries[parent].structural {
			t.Fatalf("parent = %+v, want the services copy to be structural", s.entries[parent])
		}
		// @happynetbox.com is unpinned here, so its structural copy must not
		// claim to be bookmarked — the b hint would read "remove" while
		// pressing b would add it.
		if s.entries[parent].bookmarked {
			t.Fatalf("parent = %+v, want an unpinned structural parent", s.entries[parent])
		}
		if parent+1 >= len(s.entries) {
			t.Fatalf("parent is the last row; want its children to follow")
		}
		if got := s.entries[parent+1]; got.target != "1@happynetbox.com" || !got.child {
			t.Fatalf("row after the parent = %+v, want 1@happynetbox.com as a child", got)
		}
	}
	if !foundServices {
		t.Fatal("SERVICES section not found")
	}
}

// A pinned parent keeps heading its group — structure is not a listing — but it
// must know it is bookmarked, or the b hint lies about what the key does.
func TestPinnedParentKeepsHeadingItsGroupAndKnowsItIsPinned(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{targets: []string{"@bbs.airandwave.net"}})
	services := sectionTargets(t, sections, sectionServices)
	if services[0] != "@bbs.airandwave.net" {
		t.Fatalf("services[0] = %q, want the parent retained", services[0])
	}
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if !s.entries[0].structural || !s.entries[0].bookmarked {
			t.Fatalf("parent = %+v, want structural and bookmarked", s.entries[0])
		}
		if !s.entries[1].child || s.entries[1].target != "dict@bbs.airandwave.net" {
			t.Fatalf("services[1] = %+v, want dict still a child", s.entries[1])
		}
	}
}

func TestBookmarkSectionEntriesAreMarkedBookmarked(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{targets: []string{"@tilde.team"}})
	found := false
	for _, s := range sections {
		if s.id != sectionBookmarks {
			continue
		}
		found = true
		if !s.entries[0].bookmarked {
			t.Fatalf("bookmark row = %+v, want bookmarked", s.entries[0])
		}
	}
	if !found {
		t.Fatal("BOOKMARKS section not found")
	}
}

// The delegate renders one item at a time and cannot see whether the next row
// shares a host, so the connector's shape is decided here.
func TestLastChildMarksTheFinalChildOfEveryGroup(t *testing.T) {
	sections := buildSections(loadCatalog(), bookmarkFile{})
	want := map[string]bool{
		"wordsearch:today@bbs.airandwave.net": true,  // final child of a six-child group
		"dict@bbs.airandwave.net":             false, // first child of the same group
		"calendar@flanigan.us":                true,  // final child of a two-child group
		"bonsai@flanigan.us":                  false, // its non-final sibling
		"textfile@typed-hole.org":             true,
		"cyoa@typed-hole.org":                 false,
		"random@happynetbox.com":              true,
		"@bbs.airandwave.net":                 false, // a root is not a child
		"@graph.no":                           false, // no group at all
		"@happynetbox.com":                    false, // structural parent
	}
	seen := make(map[string]bool, len(want))
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		for _, e := range s.entries {
			if expected, ok := want[e.target]; ok {
				seen[e.target] = true
				if e.lastChild != expected {
					t.Errorf("%s lastChild = %v, want %v", e.target, e.lastChild, expected)
				}
			}
		}
	}
	for target := range want {
		if !seen[target] {
			t.Errorf("%s never appeared in SERVICES", target)
		}
	}
}

// No service host in the shipped catalog has exactly one child any more —
// @flanigan.us gained bonsai — but the rule must still hold for one, so this
// case is built rather than found.
func TestLastChildMarksTheOnlyChildOfASingleChildGroup(t *testing.T) {
	catalog := []startEntry{
		{target: "@example.com", kind: kindService, note: "Root", source: sourceCatalog},
		{target: "only@example.com", kind: kindService, note: "Only child", source: sourceCatalog},
	}
	sections := buildSections(catalog, bookmarkFile{})
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if len(s.entries) != 2 {
			t.Fatalf("entries = %+v, want a root and one child", s.entries)
		}
		if s.entries[0].lastChild {
			t.Errorf("root = %+v, want lastChild false", s.entries[0])
		}
		if !s.entries[1].child || !s.entries[1].lastChild {
			t.Errorf("only child = %+v, want child and lastChild true", s.entries[1])
		}
		return
	}
	t.Fatal("SERVICES section not found")
}
