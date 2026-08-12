package tui

type startSectionID uint8

const (
	sectionUnknown startSectionID = iota
	sectionBookmarks
	sectionCommunities
	sectionServices
)

// startSection is one titled group of startpage rows.
type startSection struct {
	id      startSectionID
	title   string
	entries []startEntry
}

// buildSections merges the two sources into the rendered order: the user's
// bookmarks first, then the catalog grouped by kind.
//
// Two behaviours make bookmarking read as "pin to the top":
//   - a bookmarked target is suppressed from its catalog section rather than
//     appearing twice, and
//   - it keeps the catalog's note, so pinning never costs the description.
//
// The bookmark file stores targets only. A catalog match supplies its authored
// metadata; an unmatched target stays unclassified with a blank description.
func buildSections(catalog []startEntry, bm bookmarkFile) []startSection {
	byTarget := make(map[string]startEntry, len(catalog))
	for _, e := range catalog {
		byTarget[e.target] = e
	}

	var sections []startSection

	if len(bm.targets) > 0 {
		bookmarked := make([]startEntry, 0, len(bm.targets))
		for _, target := range bm.targets {
			e := startEntry{target: target, source: sourceBookmark}
			if catalogEntry, ok := byTarget[target]; ok {
				e = catalogEntry
				e.source = sourceBookmark
			}
			bookmarked = append(bookmarked, e)
		}
		sections = append(sections, startSection{id: sectionBookmarks, title: "BOOKMARKS", entries: bookmarked})
	}

	if bm.catalogHidden {
		return sections
	}

	pinned := make(map[string]bool, len(bm.targets))
	for _, target := range bm.targets {
		pinned[target] = true
	}
	for _, group := range []struct {
		title string
		kind  entryKind
		id    startSectionID
	}{
		{title: "COMMUNITIES", kind: kindCommunity, id: sectionCommunities},
		{title: "SERVICES", kind: kindService, id: sectionServices},
		{title: "PEOPLE", kind: kindPerson, id: sectionUnknown},
	} {
		var entries []startEntry
		for _, e := range catalog {
			if e.kind == group.kind && !pinned[e.target] {
				entries = append(entries, e)
			}
		}
		if len(entries) > 0 {
			sections = append(sections, startSection{id: group.id, title: group.title, entries: entries})
		}
	}
	return sections
}
