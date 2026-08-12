package tui

import "testing"

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
