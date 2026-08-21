package tui

import (
	"reflect"
	"slices"
	"testing"
	"time"
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

func TestBuildSectionsCopiesVisitedAfterTheCatalogOverlay(t *testing.T) {
	want := time.Date(2026, 8, 11, 16, 0, 0, 0, time.UTC)
	bm := bookmarkFile{
		targets: []string{"@tilde.team", "@new.example"},
		visited: map[string]time.Time{"@tilde.team": want},
	}
	got := buildSections(catalogFixture(), bm)
	if len(got) == 0 || got[0].title != "BOOKMARKS" || len(got[0].entries) != 2 {
		t.Fatalf("BOOKMARKS = %+v, want two pins", got)
	}
	matched := got[0].entries[0]
	if matched.target != "@tilde.team" {
		t.Fatalf("first pin = %q, want the catalog match", matched.target)
	}
	if !matched.visited.Equal(want) {
		t.Errorf("catalog pin visited = %v, want %v (must be copied after the catalog overlay)", matched.visited, want)
	}
	if matched.note == "" {
		t.Error("catalog pin lost its note; the overlay must still run")
	}
	unmatched := got[0].entries[1]
	if unmatched.target != "@new.example" {
		t.Fatalf("second pin = %q, want the unmatched target", unmatched.target)
	}
	if !unmatched.visited.IsZero() {
		t.Errorf("unvisited pin visited = %v, want zero", unmatched.visited)
	}
}

// bookmarkTargets reads the BOOKMARKS section's targets in rendered order.
func bookmarkTargets(t *testing.T, sections []startSection) []string {
	t.Helper()
	if len(sections) == 0 || sections[0].id != sectionBookmarks {
		t.Fatalf("sections = %+v, want BOOKMARKS first", sections)
	}
	out := make([]string, 0, len(sections[0].entries))
	for _, e := range sections[0].entries {
		out = append(out, e.target)
	}
	return out
}

func TestBuildSectionsOrdersBookmarksOldestVisitFirst(t *testing.T) {
	bm := bookmarkFile{
		targets: []string{"@plan.cat", "@tilde.team", "quake@bbs.airandwave.net"},
		visited: map[string]time.Time{
			"@plan.cat":                time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC),
			"@tilde.team":              time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			"quake@bbs.airandwave.net": time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
		},
	}
	got := bookmarkTargets(t, buildSections(catalogFixture(), bm))
	want := []string{"@tilde.team", "quake@bbs.airandwave.net", "@plan.cat"}
	if !slices.Equal(got, want) {
		t.Errorf("bookmark order = %v, want %v (longest ago first)", got, want)
	}
}

func TestBuildSectionsPutsNeverVisitedBookmarksLastInAlphabeticalOrder(t *testing.T) {
	bm := bookmarkFile{
		targets: []string{"@never.example", "@plan.cat", "@also-never.example", "@tilde.team"},
		visited: map[string]time.Time{
			"@plan.cat":   time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC),
			"@tilde.team": time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}
	got := bookmarkTargets(t, buildSections(catalogFixture(), bm))
	want := []string{"@tilde.team", "@plan.cat", "@also-never.example", "@never.example"}
	if !slices.Equal(got, want) {
		t.Errorf("bookmark order = %v, want %v (undated last, alphabetical among themselves)", got, want)
	}
}

func TestBuildSectionsSortsBookmarksVisitedTogetherAlphabetically(t *testing.T) {
	same := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	bm := bookmarkFile{
		targets: []string{"quake@bbs.airandwave.net", "@tilde.team", "@plan.cat", "help@bbs.airandwave.net"},
		visited: map[string]time.Time{
			"quake@bbs.airandwave.net": same,
			"@tilde.team":              same,
			"@plan.cat":                same,
			"help@bbs.airandwave.net":  same,
		},
	}
	got := bookmarkTargets(t, buildSections(catalogFixture(), bm))
	want := []string{"help@bbs.airandwave.net", "quake@bbs.airandwave.net", "@plan.cat", "@tilde.team"}
	if !slices.Equal(got, want) {
		t.Errorf("bookmark order = %v, want %v (equal dates tie-break on host, then token)", got, want)
	}
}

func TestBuildSectionsSortsAcceptedTargetFormsByParsedHostAndQuery(t *testing.T) {
	bm := bookmarkFile{
		targets: []string{
			"@z.example",
			"a.example/charlie",
			"bob@a.example",
			"alice@destination.example@relay.example",
			"finger://a.example/alice",
		},
	}
	got := bookmarkTargets(t, buildSections(catalogFixture(), bm))
	want := []string{
		"finger://a.example/alice",
		"bob@a.example",
		"a.example/charlie",
		"alice@destination.example@relay.example",
		"@z.example",
	}
	if !slices.Equal(got, want) {
		t.Errorf("bookmark order = %v, want %v (parsed host, then query)", got, want)
	}
}

func TestBuildSectionsSortManualKeepsFileOrder(t *testing.T) {
	bm := bookmarkFile{
		sortManual: true,
		targets:    []string{"@plan.cat", "@never.example", "@tilde.team"},
		visited: map[string]time.Time{
			"@plan.cat":   time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC),
			"@tilde.team": time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}
	got := bookmarkTargets(t, buildSections(catalogFixture(), bm))
	want := []string{"@plan.cat", "@never.example", "@tilde.team"}
	if !slices.Equal(got, want) {
		t.Errorf("bookmark order = %v, want %v (sort manual keeps the file's order)", got, want)
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
		"wiki@bbs.airandwave.net",
		"wordsearch:today@bbs.airandwave.net",
		"@crossed-fingers.andros.dev",
		"help@crossed-fingers.andros.dev",
		"search?gemini@crossed-fingers.andros.dev",
		"@flanigan.us",
		"bonsai@flanigan.us",
		"calendar@flanigan.us",
		"@graph.no",
		"liverpool@graph.no",
		"@happynetbox.com",
		"1@happynetbox.com",
		"bot@happynetbox.com",
		"browserversion@happynetbox.com",
		"originsfinger@happynetbox.com",
		"random@happynetbox.com",
		"@typed-hole.org",
		"cyoa@typed-hole.org",
		"lobsters@typed-hole.org",
		"smog@typed-hole.org",
		"textfile@typed-hole.org",
	}
	if got := sectionTargets(t, sections, sectionServices); !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %v, want %v", got, want)
	}
}

// @graph.no has a root and no children, so it is a plain row: no indent.
func TestRootWithoutChildrenIsNotAParent(t *testing.T) {
	sections := buildSections([]startEntry{{target: "@standalone.org", kind: kindService, note: "Standalone"}}, bookmarkFile{})
	found := false
	for _, s := range sections {
		for _, e := range s.entries {
			if e.target != "@standalone.org" {
				continue
			}
			found = true
			if e.child || e.structural {
				t.Fatalf("@standalone.org = %+v; want a plain row", e)
			}
		}
	}
	if !found {
		t.Fatal("@standalone.org not found in any section")
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
		"wiki@bbs.airandwave.net":             false,
		"lobsters@typed-hole.org":             false,
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

func TestRemovedWeatherServiceBookmarkStaysTargetOnly(t *testing.T) {
	const target = "weather@bbs.airandwave.net"
	got := buildSections(loadCatalog(), bookmarkFile{targets: []string{target}})
	if len(got) == 0 || got[0].id != sectionBookmarks || len(got[0].entries) != 1 {
		t.Fatalf("sections = %+v, want one BOOKMARKS entry", got)
	}
	entry := got[0].entries[0]
	if entry.target != target || !entry.bookmarked {
		t.Fatalf("bookmark = %+v, want retained target %q marked bookmarked", entry, target)
	}
	if entry.note != "" || entry.kind != kindUnknown {
		t.Fatalf("bookmark = %+v, want blank note and kindUnknown after catalog removal", entry)
	}
}

// dualRoleCatalog is a host listed as a community that also heads a group of
// services, plus the "group" line describing that group.
func dualRoleCatalog() []startEntry {
	return []startEntry{
		{target: "@dual.example", kind: kindCommunity, note: "Community note", source: sourceCatalog},
		{target: "@dual.example", kind: kindGroup, note: "Group note", source: sourceCatalog},
		{target: "bot@dual.example", kind: kindService, note: "Child note", source: sourceCatalog},
	}
}

// A structural parent stands in for a listing that lives in another section, so
// repeating that listing's note describes the wrong thing. A "group" line gives
// the header its own words without touching the listing.
func TestGroupNoteDescribesTheStructuralParent(t *testing.T) {
	for _, s := range buildSections(dualRoleCatalog(), bookmarkFile{}) {
		switch s.id {
		case sectionCommunities:
			if len(s.entries) != 1 || s.entries[0].note != "Community note" {
				t.Fatalf("communities = %+v, want the listing to keep its own note", s.entries)
			}
		case sectionServices:
			if len(s.entries) != 2 {
				t.Fatalf("services = %+v, want a structural parent and one child", s.entries)
			}
			parent := s.entries[0]
			if !parent.structural || parent.target != "@dual.example" {
				t.Fatalf("services[0] = %+v, want a structural @dual.example parent", parent)
			}
			if parent.note != "Group note" {
				t.Fatalf("parent note = %q, want the group note", parent.note)
			}
			if !s.entries[1].child || s.entries[1].target != "bot@dual.example" {
				t.Fatalf("services[1] = %+v, want the child row", s.entries[1])
			}
		}
	}
}

// A group line is metadata, not a listing: it must never become a row of its
// own, in any section, pinned or not.
func TestGroupLineIsNeverARowOfItsOwn(t *testing.T) {
	for _, tt := range []struct {
		name string
		bm   bookmarkFile
	}{
		// Unpinned: the community listing, the structural parent, the child.
		// Pinned: the listing moves to BOOKMARKS, parent and child stay.
		{name: "unpinned", bm: bookmarkFile{}},
		{name: "pinned", bm: bookmarkFile{targets: []string{"@dual.example"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := 0
			for _, s := range buildSections(dualRoleCatalog(), tt.bm) {
				for _, e := range s.entries {
					if e.kind == kindGroup {
						t.Errorf("section %v renders a group line: %+v", s.id, e)
					}
					rows++
				}
			}
			if rows != 3 {
				t.Errorf("rows = %d, want 3", rows)
			}
		})
	}
}

// Pinning a dual-role host moves its listing to BOOKMARKS. The listing's note
// travels with it; the group note belongs to the header it was written for.
func TestPinnedDualRoleHostKeepsItsListingNote(t *testing.T) {
	sections := buildSections(dualRoleCatalog(), bookmarkFile{targets: []string{"@dual.example"}})
	if sections[0].id != sectionBookmarks || len(sections[0].entries) != 1 {
		t.Fatalf("sections[0] = %+v, want one BOOKMARKS entry", sections[0])
	}
	if got := sections[0].entries[0].note; got != "Community note" {
		t.Fatalf("bookmark note = %q, want the listing's note", got)
	}
	for _, s := range sections {
		if s.id != sectionServices {
			continue
		}
		if got := s.entries[0].note; got != "Group note" {
			t.Fatalf("parent note = %q, want the group note even while pinned", got)
		}
	}
}

// A pinned service root is structural for the same reason, so a group note
// applies there too.
func TestGroupNoteAppliesToAPinnedServiceRoot(t *testing.T) {
	catalog := []startEntry{
		{target: "@svc.example", kind: kindService, note: "Root note", source: sourceCatalog},
		{target: "@svc.example", kind: kindGroup, note: "Group note", source: sourceCatalog},
		{target: "a@svc.example", kind: kindService, note: "Child note", source: sourceCatalog},
	}
	for _, s := range buildSections(catalog, bookmarkFile{targets: []string{"@svc.example"}}) {
		if s.id != sectionServices {
			continue
		}
		if !s.entries[0].structural || s.entries[0].note != "Group note" {
			t.Fatalf("parent = %+v, want a structural row carrying the group note", s.entries[0])
		}
		return
	}
	t.Fatal("SERVICES section not found")
}

// Without a group line, a structural parent still inherits the root's note —
// the behaviour every other grouped host relies on.
func TestStructuralParentWithoutAGroupNoteKeepsTheRootNote(t *testing.T) {
	catalog := []startEntry{
		{target: "@dual.example", kind: kindCommunity, note: "Community note", source: sourceCatalog},
		{target: "bot@dual.example", kind: kindService, note: "Child note", source: sourceCatalog},
	}
	for _, s := range buildSections(catalog, bookmarkFile{}) {
		if s.id != sectionServices {
			continue
		}
		if got := s.entries[0].note; got != "Community note" {
			t.Fatalf("parent note = %q, want the root's note", got)
		}
		return
	}
	t.Fatal("SERVICES section not found")
}

// The one dual-role host the shipped catalog actually has.
func TestHappynetboxHeadsItsServicesInItsOwnWords(t *testing.T) {
	const (
		listing = ".plan files updated via the web"
		header  = "Not just .plan files"
	)
	seen := 0
	for _, s := range buildSections(loadCatalog(), bookmarkFile{}) {
		for _, e := range s.entries {
			if e.target != "@happynetbox.com" {
				continue
			}
			seen++
			want := listing
			if s.id == sectionServices {
				want = header
			}
			if e.note != want {
				t.Errorf("section %v note = %q, want %q", s.id, e.note, want)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("@happynetbox.com rendered %d times, want a listing and a group header", seen)
	}
}
