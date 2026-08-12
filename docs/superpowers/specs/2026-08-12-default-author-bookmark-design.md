# Default author bookmark

**Date:** 2026-08-12
**Status:** approved for implementation

## Goal

Give anyone whose bookmarks file has not yet been created
`jonathan@tilde.team` as an ordinary bookmark while preserving the existing
promise that users can edit and remove bookmarks directly. This includes an
upgrade from an earlier lookit release when the user has only browsed the
catalog and never created a bookmarks file.

## Behaviour

When the resolved bookmarks file does not exist, lookit creates it with exactly
one line and a trailing newline:

```text
jonathan@tilde.team
```

The seed bytes come from `appendBookmarkLine(nil, aboutFingerAuthor)`, reusing
the author address that already powers About's finger action. The file is
fully staged at `0600` in its `0700` parent directory, then published with an
atomic create-if-absent hard link. This shares staging with the existing atomic
replacement writer without giving initialization its replacement semantics.
The newly written data is then parsed through the normal bookmark parser and
appears in `BOOKMARKS`.

Any existing bookmarks file is authoritative and remains byte-for-byte
untouched, including an empty, comment-only, or `catalog off`-only file.
Removing the default bookmark's line therefore leaves an existing empty file,
and later starts do not restore the address. Removing the file itself makes it
absent and intentionally triggers first-file initialization again. Existing
users who already have a bookmarks file do not receive the new default.

If resolving the path fails, behaviour is unchanged. If creating the initial
file fails, lookit does not invent an in-memory bookmark that cannot be removed;
it returns zero targets with a file-level `cannot create: <error>` problem, which
the startpage surfaces through its existing notice path. If another process
creates the final path after lookit observes it missing, that file wins: lookit
reads and parses it instead of replacing it. A failure to read that winning file
is surfaced as `cannot read: <error>`.

## Relation to prior specs

This is a narrow override of two decisions in the 2026-08-11 bookmarks design:
defaults were not seeded into the user's file, and the shipped catalog contained
no personal addresses.

Seeding one ordinary bookmark does not freeze a catalog snapshot: the address
has no catalog metadata or maintainer-controlled lifecycle after creation, and
the user can edit or remove it using the existing file semantics. It is also not
a catalog person entry; deleting its bookmark line removes it from the
startpage, rather than revealing another copy under `PEOPLE`.

About already exposes `jonathan@tilde.team` as lookit's author finger action.
The seed makes that same product identity a first-file startpage affordance; it
does not introduce a second identity or an inferred biographical claim.

## Scope

The change belongs at the missing-file branch of `loadBookmarks`: construct the
seed with `appendBookmarkLine(nil, aboutFingerAuthor)`, stage it through the
writer shared with `saveBookmarkData`, and publish it exclusively. A losing
publisher reads the authoritative winner. Normal bookmark edits retain
`saveBookmarkData`'s final-symlink-aware atomic replacement path. This does not
change the bookmarks grammar, catalog, section assembly, routing, or bookmark
toggle behaviour. `jonathan@tilde.team` remains an unclassified bookmark with
no catalog-authored description.

The README will say that lookit creates this initial bookmark only when the
bookmarks file is absent and that removing its line is permanent while the file
continues to exist. The stale `loadBookmarks` comment and the repository
architecture guidance will also record the first-file write.

## Tests

Tests will establish that:

- a missing bookmarks file is created and loaded with the author address;
- the created file and parent directory use `0600` and `0700` permissions;
- existing empty, non-empty, comment-only, and `catalog off`-only files remain
  byte-for-byte unchanged;
- deleting the seeded line and reloading does not recreate it;
- initialization failures surface `cannot create: <error>` with zero targets;
- a deterministic real-filesystem publication race cannot clobber the winner;
- losing publication reads the winner, and a failed winner read surfaces
  `cannot read: <error>`; and
- path-resolution failures retain their current behaviour.

The package-wide `TestMain` fixture will point at one existing empty file, and
`useTempBookmarks` will create an empty file before returning its path. This
keeps unrelated TUI and bookmark-write tests isolated from first-file
initialization. Tests of the initialization branch will use a dedicated path
that is deliberately not pre-created. The former missing-file-is-empty test is
replaced by the seed-creation test.
