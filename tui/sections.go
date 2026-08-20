package tui

import (
	"sort"
	"strings"
)

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

// sortByVisit orders dated rows longest-ago first and undated rows after them,
// breaking ties alphabetically (targetLess) so the shelf reads the same however
// the file happens to be arranged. Display-only: the file is not rewritten.
func sortByVisit(entries []startEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].visited, entries[j].visited
		if a.IsZero() != b.IsZero() {
			return !a.IsZero()
		}
		if !a.Equal(b) {
			return a.Before(b)
		}
		return targetLess(entries[i].target, entries[j].target)
	})
}

// targetLess is the alphabetical order used everywhere targets are sorted for
// display: host first, then the query token, so one host's rows stay adjacent.
func targetLess(left, right string) bool {
	leftHost, rightHost := entryHost(left), entryHost(right)
	if leftHost != rightHost {
		return leftHost < rightHost
	}
	return entryToken(left) < entryToken(right)
}

// buildSections merges the two sources into the rendered order: the user's
// bookmarks first, then the catalog grouped by kind.
//
// Two behaviours make bookmarking read as "pin to the top":
//   - a bookmarked target is suppressed from its catalog section rather than
//     appearing twice, and
//   - it carries the catalog's note with it, so unpinning restores the
//     description and "/" still matches on it. The note is data the row keeps,
//     not text the row shows: on the user's own shelf the note column is left
//     to the row itself, and startRowNote (start.go) is the one place that
//     decides so.
//
// The bookmark file stores targets and an optional last-visited date. A catalog
// match supplies its authored metadata; an unmatched target stays unclassified
// with a blank description. The date is copied after that overlay so it is not
// wiped by the catalog entry — and only then can the shelf be ordered by it
// (sortByVisit, unless the file says "sort manual").
func buildSections(catalog []startEntry, bm bookmarkFile) []startSection {
	// Group lines are excluded from every listing map: they describe a header,
	// not a place, so a bookmark must never inherit one.
	byTarget := make(map[string]startEntry, len(catalog))
	for _, e := range catalog {
		if e.kind == kindGroup {
			continue
		}
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
			e.bookmarked = true
			e.visited = bm.visited[target] // zero when unknown — the row renders blank
			bookmarked = append(bookmarked, e)
		}
		if !bm.sortManual {
			sortByVisit(bookmarked)
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

	roots := make(map[string]startEntry, len(catalog))
	groupNotes := make(map[string]string)
	for _, e := range catalog {
		if entryToken(e.target) != "" {
			continue
		}
		if e.kind == kindGroup {
			groupNotes[entryHost(e.target)] = e.note
			continue
		}
		roots[entryHost(e.target)] = e
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
		var listed []startEntry
		for _, e := range catalog {
			if e.kind == group.kind && !pinned[e.target] {
				listed = append(listed, e)
			}
		}
		if len(listed) == 0 {
			continue
		}
		if group.kind == kindService {
			listed = groupByHost(listed, roots, groupNotes, pinned)
		} else {
			sort.SliceStable(listed, func(i, j int) bool {
				return targetLess(listed[i].target, listed[j].target)
			})
		}
		sections = append(sections, startSection{id: group.id, title: group.title, entries: listed})
	}
	return sections
}

// groupByHost orders service rows by host, then by query token within each host,
// with the host's root row first. Display order is therefore computed, not
// inherited from catalog.txt: a new catalog line can be added anywhere.
//
// A host with service children whose root is not itself a listed service row —
// because the root is classified differently (@happynetbox.com is a community)
// or because it is pinned — gets a structural copy of that root as its parent.
// Structure is not a listing: structural rows are not counted and vanish while
// filtering. Because such a row heads a group rather than offering the place it
// names, a "group" line in the catalog may give it its own note; without one it
// inherits the root's, which is what every single-role host relies on.
func groupByHost(listed []startEntry, roots map[string]startEntry, groupNotes map[string]string, pinned map[string]bool) []startEntry {
	byHost := make(map[string][]startEntry, len(listed))
	var hosts []string
	for _, e := range listed {
		host := entryHost(e.target)
		if _, seen := byHost[host]; !seen {
			hosts = append(hosts, host)
		}
		byHost[host] = append(byHost[host], e)
	}
	sort.Strings(hosts)

	out := make([]startEntry, 0, len(listed)+len(hosts))
	for _, host := range hosts {
		rows := byHost[host]
		// A root's token is "", which sorts before every child.
		sort.SliceStable(rows, func(i, j int) bool {
			return entryToken(rows[i].target) < entryToken(rows[j].target)
		})

		hasChild := false
		for _, e := range rows {
			if entryToken(e.target) != "" {
				hasChild = true
				break
			}
		}
		root, hasRoot := roots[host]
		// No children means no group. No root means the catalog invariant is
		// broken (TestCatalogHasRootForEveryGroupedHost); render flat rather
		// than inventing a parent, so the rows stay reachable.
		if !hasChild || !hasRoot {
			out = append(out, rows...)
			continue
		}

		if rows[0].target == root.target {
			out = append(out, rows[0])
			rows = rows[1:]
		} else {
			parent := root
			parent.structural = true
			parent.bookmarked = pinned[root.target]
			if note, ok := groupNotes[host]; ok {
				parent.note = note
			}
			out = append(out, parent)
		}
		for i, e := range rows {
			e.child = true
			e.lastChild = i == len(rows)-1
			out = append(out, e)
		}
	}
	return out
}

// entryHost is the address after the final "@": the machine a row belongs to.
// Grouping keys off this, so "@graph.no" and "oslo@graph.no" group together.
func entryHost(target string) string {
	if i := strings.LastIndex(target, "@"); i >= 0 {
		return target[i+1:]
	}
	return target
}

// entryToken is the query before the final "@" — empty for a host root, which
// is what makes a row a parent rather than a child.
func entryToken(target string) string {
	if i := strings.LastIndex(target, "@"); i >= 0 {
		return target[:i]
	}
	return ""
}
