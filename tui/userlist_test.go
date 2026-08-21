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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"klu", "tomasino"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (ships block must be excluded)", got, want)
	}
}

func TestParseGridSingleUserZaibatsu(t *testing.T) {
	body := []byte("Currently logged in sundogs:\ndokuja\n")
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"andypiper", "benbrown", "goose"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (command line excluded)", got, want)
	}
}

func TestParseGenericReviewFixture(t *testing.T) {
	// Must stay in lockstep with docs/tui-review/fixtures/fingerd genericBody.
	parsed, ok := parseUserList([]byte("carol  Carol Review\ndave  Dave Review\n"), "")
	if !ok {
		t.Fatal("parseUserList ok = false, want a generic list")
	}
	if !parsed.generic {
		t.Fatal("generic = false, want true (no named-format cue)")
	}
	if got, want := logins(parsed.users), []string{"carol", "dave"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
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
	users, ok := ParseUsers(body, "")
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
			parsed, ok := parseUserList(tt.body, "")
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
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (banner only)")
	}
}

func TestDeclineEmptyTildeClub(t *testing.T) {
	if _, ok := ParseUsers([]byte(""), ""); ok {
		t.Fatal("ParseUsers ok = true, want false (empty)")
	}
}

func TestDeclineInlineCueTypedHoleWithoutAvailableFingers(t *testing.T) {
	// Users are inline on the cue line ("probably julien"); must NOT be parsed.
	body := []byte("Welcome to the Typed Hole\n" +
		"Users currently logged in: probably julien\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (inline cue must not parse)")
	}
}

func TestDeclineDaemonHelpDebian(t *testing.T) {
	body := []byte("userdir-ldap finger daemon\n--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n  where uid is the user id\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (daemon help)")
	}
}

func TestDeclineGraphNoWeatherHelp(t *testing.T) {
	// graph.no bare-host help: prose + "finger oslo@graph.no" usage example.
	// Must decline (oslo is a placeholder, not a user); additive-only rule.
	body := []byte("Weather via finger, graph.no\n\n" +
		"* Contact: finger@falkp.no\n\n" +
		"Usage:\n    finger oslo@graph.no\n\n" +
		"Using imperial units:\n    finger ^oslo@graph.no\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (graph.no usage help)")
	}
}

func TestDeclineDebianAttributeLegendFull(t *testing.T) {
	// Full db.debian.org attribute legend (10 "key : value" lines). Guards the
	// colon-form exclusion in the generic fallback.
	body := []byte("userdir-ldap finger daemon\n--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n" +
		"    The following attributes are currently supported:\n" +
		"      cn : First name\n      mn : Middle name\n      sn : Last name\n" +
		"      email : Email\n      labeleduri : URL\n      ircnick : IRC nickname\n" +
		"      icquin : ICQ UIN\n      jabberjid : Jabber ID\n" +
		"      keyfingerprint : Fingerprint\n      key : Key block\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (LDAP attribute legend)")
	}
}

// The menu/table matchers are gated by a cue; a cue with no parseable entries
// must still decline rather than open an empty or hallucinated list.

func TestDeclineAvailableFingersCueWithoutEntries(t *testing.T) {
	body := []byte("Welcome.\n\n<== Available Fingers ==>\n\n" +
		"(the service list is temporarily unavailable)\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (Available Fingers cue, no entries)")
	}
}

func TestDeclineTelehackHeaderWithoutRows(t *testing.T) {
	body := []byte("TELEHACK SYSTEM STATUS\n\n" +
		" port username   status\n ---- --------   ------\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (telehack header, no data rows)")
	}
}

func TestDeclineRingCueWithoutURLs(t *testing.T) {
	body := []byte("This is the finger ring!\nand now for the list:\n" +
		"the ring is empty today, check back soon\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (ring cue, no finger:// URLs)")
	}
}

func TestDeclineSavaTitleWithoutTableRows(t *testing.T) {
	body := []byte("Users on this finger server\n\n(none connected right now)\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (sava title, no | table rows)")
	}
}

// --- Generic fallback: structured-login gate ---

func TestParseGenericBareLoginBlock(t *testing.T) {
	// No Login header, no online/logged-in cue, no "> " marker, no named menu:
	// every earlier matcher declines, so the generic fallback must open this.
	body := []byte("the crew:\nbetsy\nMelchizedek\nOleander\nStarbloom\n")
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true (bare-login block)")
	}
	want := []string{"betsy", "Melchizedek", "Oleander", "Starbloom"}
	if got := logins(users); !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseGenericColumnarNoHeader(t *testing.T) {
	// login + 2-space gap + name, but no "Login" header so parseColumnar declines.
	body := []byte("klu      pilot\ntomasino  navigator\n")
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true (headerless columnar)")
	}
	if got, want := logins(users), []string{"klu", "tomasino"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if users[0].Name != "pilot" || users[1].Name != "navigator" {
		t.Fatalf("names = %q,%q want pilot,navigator", users[0].Name, users[1].Name)
	}
}

func TestGenericRequiresTwoLogins(t *testing.T) {
	// A single bare-login line is not enough to open a list.
	if _, ok := ParseUsers([]byte("Welcome.\n\nbetsy\n"), ""); ok {
		t.Fatal("ParseUsers ok = true, want false (only one structured login)")
	}
}

func TestGenericDeclinesColonLegendDebian(t *testing.T) {
	// db.debian.org daemon help: a "key : value" attribute legend must NOT be
	// read as a user list. This is the headline guard for excluding the colon
	// (and single-space) form from the generic fallback.
	body := []byte("userdir-ldap finger daemon\n" +
		"--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n" +
		"    The following attributes are currently supported:\n" +
		"      cn : First name\n" +
		"      mn : Middle name\n" +
		"      sn : Last name\n" +
		"      email : Email\n" +
		"      key : Key block\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (colon attribute legend)")
	}
}

func TestGenericDeclinesSingleSpaceProse(t *testing.T) {
	// "login value" with a single space is prose, not a columnar entry.
	body := []byte("must provide username\nplease try again later\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (single-space prose)")
	}
}

// --- Generic fallback: additive strong-context targets ---

func TestGenericHarvestsFingerCommandTarget(t *testing.T) {
	// A bare-login block opens the list; a "finger user@host" mention elsewhere
	// in the body is appended as a cross-host drill target.
	body := []byte("betsy\nMelchizedek\n\nContact me: finger bob@example.org\n")
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"betsy", "Melchizedek", "bob"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (harvested target appended last)", got, want)
	}
	if users[2].Target != "bob@example.org" {
		t.Fatalf("users[2].Target = %q, want bob@example.org", users[2].Target)
	}
}

func TestGenericHarvestsFingerURLTarget(t *testing.T) {
	// A bare-login block opens the list; a finger:// URL elsewhere in the body
	// is appended as a cross-host drill target (login@host, host as the name).
	body := []byte("betsy\nMelchizedek\n\nsee also finger://example.org/carol\n")
	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"betsy", "Melchizedek", "carol"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if users[2].Target != "carol@example.org" {
		t.Fatalf("users[2].Target = %q, want carol@example.org", users[2].Target)
	}
	if users[2].Name != "example.org" {
		t.Fatalf("users[2].Name = %q, want example.org", users[2].Name)
	}
}

func TestStrongGate_ServiceQueriesStayReaderOnly(t *testing.T) {
	body := []byte(
		"Login   Name\n" +
			"alice   Alice\n" +
			"bob     Bob\n" +
			"\n" +
			"Use: finger dict:word@other.host\n" +
			"Use: finger urban:old school@bbs.airandwave.net\n",
	)
	parsed, ok := parseUserList(body, "example.com:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	harvested := appendHarvestedTargets(parsed.users, body, "example.com:79")
	if got := logins(harvested); !reflect.DeepEqual(got, []string{"alice", "bob"}) {
		t.Fatalf("logins = %v, want only structured rows", got)
	}
	if loginRe.MatchString("dict:word") || loginRe.MatchString("urban:old school") {
		t.Fatal("service queries unexpectedly match the harvestable login grammar")
	}

	links := DetectLinks(body, "example.com:79")
	if link, ok := findLink(links, "dict:word@other.host"); !ok || !link.Strong || link.Action != ActionDrill {
		t.Fatalf("dict reader link = %#v, found=%v", link, ok)
	}
	if link, ok := findLink(links, "urban:old school@bbs.airandwave.net"); !ok || !link.Strong || link.Action != ActionDrill {
		t.Fatalf("urban reader link = %#v, found=%v", link, ok)
	}
}

func TestGenericTargetsDoNotOpenAlone(t *testing.T) {
	// No structured-login block: a lone "finger user@host" mention in prose must
	// NOT open a list (additive-only rule). This is the graph.no shape.
	body := []byte("Weather via finger.\nUsage:\n    finger oslo@graph.no\n")
	if _, ok := ParseUsers(body, ""); ok {
		t.Fatal("ParseUsers ok = true, want false (targets are additive-only)")
	}
}

func TestStructuredLogin(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantLogin string
		wantName  string
		wantOK    bool
	}{
		{"bare login", "betsy", "betsy", "", true},
		{"two-space columnar", "alice  Bob Smith", "alice", "Bob Smith", true},
		{"tab columnar", "alice\tBob Smith", "alice", "Bob Smith", true},
		{"single space is prose", "alice bob", "", "", false},
		{"colon form is prose", "cn : First name", "", "", false},
		{"empty line", "", "", "", false},
		{"non-login token", "<==", "", "", false},
		{"leading whitespace bare", "   betsy", "betsy", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			login, name, ok := structuredLogin(tt.line)
			if ok != tt.wantOK || login != tt.wantLogin || name != tt.wantName {
				t.Fatalf("structuredLogin(%q) = (%q,%q,%v), want (%q,%q,%v)",
					tt.line, login, name, ok, tt.wantLogin, tt.wantName, tt.wantOK)
			}
		})
	}
}

func TestParseUsers_NoLiveEscapeInParsedFields(t *testing.T) {
	// A host user list whose display column already passed through finger
	// ingress sanitization (ESC -> "^["). ParseUsers must never resurrect a
	// raw ESC into a field the list delegate will render.
	body := []byte("Login     Name\n" +
		"alice     ^[[31mAlice^[[0m\n" +
		"bob       Bob\n")

	users, ok := ParseUsers(body, "")
	if !ok {
		t.Fatalf("ParseUsers declined a columnar list it should accept")
	}
	for _, u := range users {
		for _, field := range []string{u.Login, u.Name, u.Target} {
			if strings.ContainsRune(field, 0x1b) {
				t.Errorf("parsed field contains a raw ESC: %q", field)
			}
		}
	}
}

// ---- Address-only listings (a run of bare user@host lines) ----

func TestParseUsers_CrossedFingersSearchAllowsSingleResult(t *testing.T) {
	body := []byte("Search results for \"jonathandeamer\":\n\njonathan@tilde.team\n")

	parsed, ok := parseUserListForTarget(body, hostTarget(t, "search?jonathandeamer@crossed-fingers.andros.dev"))
	if !ok {
		t.Fatal("parseUserList ok = false, want true for one Crossed Fingers search result")
	}
	if len(parsed.users) != 1 {
		t.Fatalf("users = %#v, want one result", parsed.users)
	}
	want := User{Login: "jonathan", Target: "jonathan@tilde.team"}
	if parsed.users[0] != want {
		t.Errorf("user = %#v, want %#v", parsed.users[0], want)
	}
	if parsed.preamble != "Search results for \"jonathandeamer\":" {
		t.Errorf("preamble = %q, want the search heading", parsed.preamble)
	}
	if parsed.generic {
		t.Error("generic = true, want false for a named Crossed Fingers format")
	}
}

func TestParseUsers_CrossedFingersSearchAllowsTwoResults(t *testing.T) {
	body := []byte("Search results for \"smolnet\":\n\necho@plan.cat\nfab@redterminal.org\n")

	parsed, ok := parseUserListForTarget(body, hostTarget(t, "search?smolnet@crossed-fingers.andros.dev"))
	if !ok {
		t.Fatal("parseUserList ok = false, want true for two Crossed Fingers search results")
	}
	want := []string{"echo@plan.cat", "fab@redterminal.org"}
	if len(parsed.users) != len(want) {
		t.Fatalf("users = %#v, want targets %v", parsed.users, want)
	}
	for i, target := range want {
		if parsed.users[i].Target != target {
			t.Errorf("users[%d].Target = %q, want %q", i, parsed.users[i].Target, target)
		}
	}
}

func TestParseUsers_CrossedFingersSearchShapeIsHostScoped(t *testing.T) {
	body := []byte("Search results for \"alice\":\n\nalice@example.com\n")

	if _, ok := parseUserList(body, "unrelated.example:79"); ok {
		t.Fatal("parseUserList ok = true, want false for a single address from another host")
	}
}

func TestParseUsers_CrossedFingersNoMatchesStaysReader(t *testing.T) {
	body := []byte("Search results for \"zzzzlookitnomatchzzzz\":\n\nNo matches.\n")

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?zzzzlookitnomatchzzzz@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserList ok = true, want false for a no-match response")
	}
}

func TestParseUsers_CrossedFingersChangedLayoutDeclinesWholeResponse(t *testing.T) {
	body := []byte(
		"Search results for \"smolnet\":\n\n" +
			"echo@plan.cat\n" +
			"fab@redterminal.org\n" +
			"hyblon@net.iltabellinoweb.eu\n" +
			"smog1@typed-hole.org\n" +
			"Results may be ranked differently.\n",
	)

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?smolnet@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserList ok = true, want false for an unrecognized search layout")
	}
}

func TestParseUsers_CrossedFingersChangedLayoutDoesNotFallThroughToGenericMatcher(t *testing.T) {
	body := []byte(
		"Search results for \"smolnet\":\n\n" +
			"Login     Name\n" +
			"echo      Echo\n",
	)

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?smolnet@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserList ok = true, want the claimed search response to bypass generic matchers")
	}
}

func TestParseUsers_CrossedFingersSearchRejectsContentBeforeHeading(t *testing.T) {
	body := []byte(
		"Notice: results are experimental.\n" +
			"Search results for \"smolnet\":\n\n" +
			"echo@plan.cat\n",
	)

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?smolnet@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserListForTarget ok = true, want false for content before the search heading")
	}
}

// An aggregator lists accounts that live on other hosts, so each line is a
// whole address rather than a local login. crossed-fingers.andros.dev's
// "Registered accounts" index is the shape this matches.
func TestParseUsers_AddressList(t *testing.T) {
	body := []byte(
		"For help: finger help@crossed-fingers.andros.dev\n" +
			"\n" +
			"Registered accounts (4):\n" +
			"  0@typed-hole.org\n" +
			"  akkartik@plan.cat\n" +
			"  simonmorehouse@local\n" +
			"  ben@tilde.team\n" +
			"  tomasino@cosmic.voyage\n",
	)
	parsed, ok := parseUserList(body, "crossed-fingers.andros.dev:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true for a run of address-only lines")
	}
	wantLogins := []string{"0", "akkartik", "ben", "tomasino"}
	if got := logins(parsed.users); !reflect.DeepEqual(got, wantLogins) {
		t.Errorf("logins = %v, want %v", got, wantLogins)
	}
	// Each entry must drill to its own host, not to the aggregator.
	wantTargets := []string{"0@typed-hole.org", "akkartik@plan.cat", "ben@tilde.team", "tomasino@cosmic.voyage"}
	for i, want := range wantTargets {
		if parsed.users[i].Target != want {
			t.Errorf("users[%d].Target = %q, want %q", i, parsed.users[i].Target, want)
		}
	}
	if parsed.generic {
		t.Error("generic = true, want false: an address-only run is a recognized shape, not a guess")
	}
	if !strings.Contains(parsed.preamble, "Registered accounts (4):") {
		t.Errorf("preamble = %q, want it to keep the lines above the run", parsed.preamble)
	}
}

// Two addresses on their own lines are ordinary prose in a contact block. The
// run has to be long enough to mean a listing.
func TestParseUsers_AddressListDeclinesShortRun(t *testing.T) {
	body := []byte(
		"Questions about the server?\n" +
			"\n" +
			"admin@example.com\n" +
			"postmaster@example.com\n",
	)
	if _, ok := parseUserList(body, "example.com:79"); ok {
		t.Error("parseUserList ok = true, want false for a two-address contact block")
	}
}

// A line has to be nothing but the address. Prose around it means the address
// is a mention, not a list entry.
func TestParseUsers_AddressListDeclinesProse(t *testing.T) {
	body := []byte(
		"mail alice@example.com about accounts\n" +
			"mail bob@example.com about billing\n" +
			"mail carol@example.com about anything else\n",
	)
	if _, ok := parseUserList(body, "example.com:79"); ok {
		t.Error("parseUserList ok = true, want false when the addresses sit in prose")
	}
}

// A placeholder host with no dot is not an address.
func TestParseUsers_AddressListDeclinesPlaceholderHosts(t *testing.T) {
	body := []byte(
		"Syntax:\n" +
			"  user@host\n" +
			"  name@server\n" +
			"  login@machine\n",
	)
	if _, ok := parseUserList(body, "example.com:79"); ok {
		t.Error("parseUserList ok = true, want false for placeholder hosts with no dot")
	}
}

// A recognized local-login format still wins: the address run must not steal a
// body that an earlier matcher already understands.
func TestParseUsers_AddressListYieldsToColumnar(t *testing.T) {
	body := []byte(
		"Login   Name\n" +
			"alice   Alice Smith\n" +
			"bob     Bob Jones\n" +
			"\n" +
			"Mirrors:\n" +
			"  alice@mirror.example\n" +
			"  bob@mirror.example\n" +
			"  carol@mirror.example\n",
	)
	parsed, ok := parseUserList(body, "example.com:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	if len(parsed.users) < 2 || parsed.users[0].Login != "alice" || parsed.users[0].Target != "" {
		t.Fatalf("users = %#v, want the columnar logins first with no explicit target", parsed.users)
	}
}

// Duplicates collapse and first position is kept, matching every other matcher.
func TestParseUsers_AddressListDeduplicates(t *testing.T) {
	body := []byte(
		"  a@x.example\n" +
			"  b@y.example\n" +
			"  a@x.example\n" +
			"  c@z.example\n",
	)
	parsed, ok := parseUserList(body, "example.com:79")
	if !ok {
		t.Fatal("parseUserList ok = false, want true")
	}
	if got := logins(parsed.users); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("logins = %v, want [a b c]", got)
	}
}

// A forwarded target lands on the aggregator's HostPort but the body comes from
// the relayed host, so it must not reach the strict search parser: that parser
// makes a single address conclusive, which is only safe for a body Crossed
// Fingers itself composed.
func TestParseUsers_CrossedFingersSearchRejectsForwardedTarget(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\nbob@evil.example\ncarol@evil.example\n")
	target := hostTarget(t, "search?foo@evil.example@crossed-fingers.andros.dev")

	if isCrossedFingersSearchTarget(target) {
		t.Fatal("isCrossedFingersSearchTarget = true, want false for a forwarded target")
	}
	if _, ok := parseUserListForTarget(body, target); ok {
		t.Fatal("parseUserListForTarget ok = true, want false for a relayed body")
	}
}

func TestParseUsers_CrossedFingersSearchPrefixIsCaseInsensitive(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\nbob@plan.cat\n")
	target := hostTarget(t, "SEARCH?foo@crossed-fingers.andros.dev")

	if !isCrossedFingersSearchTarget(target) {
		t.Fatal("isCrossedFingersSearchTarget = false, want true for an upper-case prefix")
	}
	if _, ok := parseUserListForTarget(body, target); !ok {
		t.Fatal("parseUserListForTarget ok = false, want true")
	}
}

// One result lookit cannot drill must not cost the reader the ones it can.
func TestParseUsers_CrossedFingersSearchSkipsUndrillableResult(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\nbob@plan.cat\ndave@tilde.club:7979\neve@plan.cat\n")

	parsed, ok := parseUserListForTarget(body, hostTarget(t, "search?foo@crossed-fingers.andros.dev"))
	if !ok {
		t.Fatal("parseUserListForTarget ok = false, want true despite one unopenable result")
	}
	want := []string{"bob@plan.cat", "eve@plan.cat"}
	if len(parsed.users) != len(want) {
		t.Fatalf("users = %#v, want targets %v", parsed.users, want)
	}
	for i, target := range want {
		if parsed.users[i].Target != target {
			t.Errorf("users[%d].Target = %q, want %q", i, parsed.users[i].Target, target)
		}
	}
}

// A line that is not address-shaped is still fatal: the shape is the only
// evidence the body really is a result list.
func TestParseUsers_CrossedFingersSearchStillDeclinesOnProse(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\nbob@plan.cat\nSee also the index.\n")

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?foo@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserListForTarget ok = true, want false for a prose line among results")
	}
}

// Every entry unopenable leaves nothing to select, so the reader keeps the body.
func TestParseUsers_CrossedFingersSearchDeclinesWhenEveryResultUndrillable(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\nbob@local\ncarol@local\n")

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?foo@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserListForTarget ok = true, want false when no result can be drilled")
	}
}

// A login the strict grammar cannot open — over 32 chars, or carrying a
// character like '+' — is still plainly an address to a human reader. Like an
// unsane host, such a result must skip rather than take the whole list to the
// reader.
func TestParseUsers_CrossedFingersSearchSkipsUngrammarLogin(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\n" +
		"bob@plan.cat\n" +
		"echoechoechoechoechoechoechoechoecho@plan.cat\n" +
		"user+tag@plan.cat\n" +
		"eve@plan.cat\n")

	parsed, ok := parseUserListForTarget(body, hostTarget(t, "search?foo@crossed-fingers.andros.dev"))
	if !ok {
		t.Fatal("parseUserListForTarget ok = false, want true despite unopenable logins")
	}
	want := []string{"bob@plan.cat", "eve@plan.cat"}
	if len(parsed.users) != len(want) {
		t.Fatalf("users = %#v, want targets %v", parsed.users, want)
	}
	for i, target := range want {
		if parsed.users[i].Target != target {
			t.Errorf("users[%d].Target = %q, want %q", i, parsed.users[i].Target, target)
		}
	}
}

// A quoted relay template is address-like but not address-shaped; it must skip
// too, not decline the response.
func TestParseUsers_CrossedFingersSearchSkipsQuotedRelayTemplate(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\n" +
		"bob@plan.cat\n" +
		"\"tomasino@cosmic.voyage\"@relay.example\n" +
		"eve@plan.cat\n")

	parsed, ok := parseUserListForTarget(body, hostTarget(t, "search?foo@crossed-fingers.andros.dev"))
	if !ok {
		t.Fatal("parseUserListForTarget ok = false, want true despite a relay-template line")
	}
	want := []string{"bob@plan.cat", "eve@plan.cat"}
	if len(parsed.users) != len(want) {
		t.Fatalf("users = %#v, want targets %v", parsed.users, want)
	}
}

// Every entry unopenable by the strict grammar leaves nothing to select, so
// the reader keeps the body.
func TestParseUsers_CrossedFingersSearchDeclinesWhenEveryResultUngrammar(t *testing.T) {
	body := []byte("Search results for \"foo\":\n\nechoechoechoechoechoechoechoechoechoecho@plan.cat\n")

	if _, ok := parseUserListForTarget(body, hostTarget(t, "search?foo@crossed-fingers.andros.dev")); ok {
		t.Fatal("parseUserListForTarget ok = true, want false when no result fits the login grammar")
	}
}
