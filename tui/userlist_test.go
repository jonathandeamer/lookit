package tui

import (
	"reflect"
	"testing"
)

func logins(users []User) []string {
	out := make([]string, len(users))
	for i, u := range users {
		out[i] = u.Login
	}
	return out
}

// --- Hosts that should parse into a user list ---

func TestParseColumnarPlanCat(t *testing.T) {
	body := []byte("Login                Name                 Login Time\n" +
		"jss                                       Fri May 29 05:31 UTC\n" +
		"geurimja             Geurimja             Thu May 28 21:57 UTC\n" +
		"26d0                 Jimenshi             Thu May 28 03:20 UTC\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"jss", "geurimja", "26d0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if users[1].Name != "Geurimja" {
		t.Fatalf("users[1].Name = %q, want %q", users[1].Name, "Geurimja")
	}
	if users[0].Name != "" {
		t.Fatalf("users[0].Name = %q, want empty (jss has no name)", users[0].Name)
	}
}

func TestParseColumnarDedupTildePink(t *testing.T) {
	body := []byte("Login       Name                Tty      Idle  Login Time   Where\n" +
		"irek                            pts/15   207d  Sep 13 2025\n" +
		"irek                            pts/16   256d  Sep 14 2025\n" +
		"ghoti                           pts/7      1d  Apr  6 14:59\n" +
		"irek                            pts/17   200d  Sep 14 2025\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"irek", "ghoti"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (deduped, order preserved)", got, want)
	}
}

func TestParseGridTildeTeam(t *testing.T) {
	body := []byte("welcome to tilde.team\n\n" +
		"hello somehost,\n" +
		"users currently logged in are:\n\n" +
		"alrs\tdtracker\tkapad\n" +
		"anshupati\tenyc\tkneezle\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	want := []string{"alrs", "dtracker", "kapad", "anshupati", "enyc", "kneezle"}
	if got := logins(users); !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseGridStopsAtSecondBlockCosmicVoyage(t *testing.T) {
	// cosmic.voyage: the "online" block must parse; the separate
	// "Who control these ships:" block (multi-word personas) must NOT.
	body := []byte("Users currently online:\n" +
		"   klu tomasino\n\n" +
		"Who control these ships:\n" +
		"   betsy\n" +
		"   Melvin P Feltersnatch\n" +
		"   Oleander\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"klu", "tomasino"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (ships block must be excluded)", got, want)
	}
}

func TestParseGridSingleUserZaibatsu(t *testing.T) {
	body := []byte("Currently logged in sundogs:\ndokuja\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"dokuja"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseMarkerHappyNetBox(t *testing.T) {
	body := []byte("Happy Net Box\n\n25 most recently updated profiles:\n" +
		"> andypiper\n> benbrown\n> goose\n\n" +
		"For a random profile:\n> finger random@happynetbox.com\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"andypiper", "benbrown", "goose"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (command line excluded)", got, want)
	}
}

// --- Hosts that should NOT parse (decline -> plain reader) ---

func TestDeclineBannerTildeTown(t *testing.T) {
	body := []byte("Hi! we're a little community that exists on a linux server. " +
		"to learn more go to https://tilde.town\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (banner only)")
	}
}

func TestDeclineEmptyTildeClub(t *testing.T) {
	if _, ok := ParseUsers([]byte("")); ok {
		t.Fatal("ParseUsers ok = true, want false (empty)")
	}
}

func TestDeclineInlineCueTypedHole(t *testing.T) {
	// Users are inline on the cue line ("probably julien"); must NOT be parsed.
	body := []byte("Welcome to the Typed Hole\n" +
		"Users currently logged in: probably julien\n\n" +
		"Available fingers:\n" +
		"weather:\tget weather\nlobsters:\tget stories\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (inline cue must not parse)")
	}
}

func TestDeclineDaemonHelpDebian(t *testing.T) {
	body := []byte("userdir-ldap finger daemon\n--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n  where uid is the user id\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (daemon help)")
	}
}
