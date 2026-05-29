package tui

import (
	"reflect"
	"strings"
	"testing"
)

func logins(users []User) []string {
	out := make([]string, len(users))
	for i, u := range users {
		out[i] = u.Login
	}
	return out
}

func targets(users []User) []string {
	out := make([]string, len(users))
	for i, u := range users {
		out[i] = u.Target
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

func TestParseTypedHoleAvailableFingers(t *testing.T) {
	body := []byte("Welcome to the Typed Hole\n" +
		"Users currently logged in: probably julien\n\n" +
		"Available fingers:\n\n" +
		"username:\t\tget user infos\n" +
		"feed:\t\t\tget my latest toots\n" +
		"lobsters:\t\tget lobste.rs hottest stories\n" +
		"weather:\t\tget typed-hole.org current weather\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"username", "feed", "lobsters", "weather"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if users[1].Name != "get my latest toots" {
		t.Fatalf("users[1].Name = %q, want description", users[1].Name)
	}
}

func TestParseSavaRocksTable(t *testing.T) {
	body := []byte("Welcome to the @sava.rocks finger server\n\n" +
		"+--------------------------------------------------------------+\n" +
		"| Users on this finger server                                  |\n" +
		"+---------+----------------------+-----------------------------+\n" +
		"| sava    | almighty owner       | finger sava@sava.rocks      |\n" +
		"+---------+----------------------+-----------------------------+\n" +
		"| weather | weather for Braila   | finger weather@sava.rocks   |\n" +
		"+---------+----------------------+-----------------------------+\n" +
		"| root    | no linux without him | system account / no passwd  |\n" +
		"+---------+----------------------+-----------------------------+\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"sava", "weather"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if got, want := targets(users), []string{"sava@sava.rocks", "weather@sava.rocks"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestParseRedterminalAvailableFingers(t *testing.T) {
	body := []byte("Welcome to the @redterminal.org finger service.\n\n" +
		"<== Available Fingers ==>\n\n" +
		"fab      fab's contact and /now page\n" +
		"feed     @fab@pleroma.envs.net's latest Mastodon toots\n" +
		"gemlog   Get latest gemlog post\n" +
		"weather  Current weather at fab's place\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"fab", "feed", "gemlog", "weather"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseTheBackupBoxRing(t *testing.T) {
	body := []byte("This is the finger ring!\n" +
		"and now for the list:\n" +
		"=> 2026-05-25 finger://tilde.team/yalla\n" +
		"=> 2026-05-23 finger://envs.net/wheresalice\n" +
		"=> 2026-05-06 finger://thebackupbox.net/epoch\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"yalla", "wheresalice", "epoch"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if got, want := targets(users), []string{"yalla@tilde.team", "wheresalice@envs.net", "epoch@thebackupbox.net"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestParseTelehackStatusTable(t *testing.T) {
	body := []byte("TELEHACK SYSTEM STATUS  2026-May-29  06:14:42\n" +
		"109 users  load 1.07  up 87d\n\n" +
		" port username   status                last what       where\n" +
		" ---- --------   ------                ---- ----       -----\n" +
		" 0    operator   System Operator       10m             console\n" +
		" 167  -                                0s              Queens, NY\n" +
		" 182  miser      CommanderKeenVI       6s   relay      Zeeland, MI\n" +
		" 55   spoony     1step4ward2stepsback  11s  finger     Adelaide, Australia\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"operator", "miser", "spoony"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParsedListPreamblesExcludeRawSelectableRows(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		want      string
		notWanted string
	}{
		{
			name: "tilde team grid",
			body: []byte("welcome to tilde.team\n\n" +
				"users currently logged in are:\n\n" +
				"alrs\tdtracker\tkapad\n"),
			want:      "users currently logged in are:",
			notWanted: "alrs\tdtracker\tkapad",
		},
		{
			name: "typed hole menu",
			body: []byte("Welcome to the Typed Hole\n\n" +
				"Available fingers:\n\n" +
				"feed:\tget my latest toots\n"),
			want:      "Available fingers:",
			notWanted: "feed:\tget my latest toots",
		},
		{
			name: "sava table",
			body: []byte("Welcome to the @sava.rocks finger server\n\n" +
				"| Users on this finger server                                  |\n" +
				"| sava    | almighty owner       | finger sava@sava.rocks      |\n"),
			want:      "Users on this finger server",
			notWanted: "finger sava@sava.rocks",
		},
		{
			name: "redterminal menu",
			body: []byte("Welcome to the @redterminal.org finger service.\n\n" +
				"<== Available Fingers ==>\n\n" +
				"fab      fab's contact and /now page\n"),
			want:      "Available Fingers",
			notWanted: "fab      fab's contact",
		},
		{
			name: "finger ring",
			body: []byte("This is the finger ring!\n" +
				"and now for the list:\n" +
				"=> 2026-05-25 finger://tilde.team/yalla\n"),
			want:      "and now for the list:",
			notWanted: "finger://tilde.team/yalla",
		},
		{
			name: "telehack status",
			body: []byte("TELEHACK SYSTEM STATUS\n\n" +
				" port username   status                last what       where\n" +
				" ---- --------   ------                ---- ----       -----\n" +
				" 0    operator   System Operator       10m             console\n"),
			want:      "---- --------",
			notWanted: "operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseUserList(tt.body)
			if !ok {
				t.Fatal("parseUserList ok = false, want true")
			}
			if !strings.Contains(parsed.preamble, tt.want) {
				t.Fatalf("preamble = %q, want it to contain %q", parsed.preamble, tt.want)
			}
			if strings.Contains(parsed.preamble, tt.notWanted) {
				t.Fatalf("preamble = %q, must not contain raw selectable row %q", parsed.preamble, tt.notWanted)
			}
		})
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

func TestDeclineInlineCueTypedHoleWithoutAvailableFingers(t *testing.T) {
	// Users are inline on the cue line ("probably julien"); must NOT be parsed.
	body := []byte("Welcome to the Typed Hole\n" +
		"Users currently logged in: probably julien\n")
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
