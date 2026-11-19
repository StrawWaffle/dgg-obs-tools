package dgg

import (
	"strings"
	"testing"
	"time"
)

var dggjams = Channel{Platform: "kick", ID: "dggjams"}

func frame(t *testing.T, raw string) Frame {
	t.Helper()
	parsed, ok := ParseFrame(raw)
	if !ok {
		t.Fatalf("could not parse %q", raw)
	}
	return parsed
}

func nicks(watchers []Watcher) string {
	names := make([]string, len(watchers))
	for i, watcher := range watchers {
		names[i] = watcher.Nick
	}
	return strings.Join(names, ",")
}

func testCatalogue(t *testing.T) *Catalogue {
	t.Helper()
	catalogue, err := ParseCatalogue([]byte(`[
		{"prefix":"catJAM","twitch":false,"minimumSubTier":0},
		{"prefix":"pepeJAM","twitch":false,"minimumSubTier":0},
		{"prefix":"cat","twitch":false,"minimumSubTier":0},
		{"prefix":"PepoTurkey","twitch":false,"minimumSubTier":2},
		{"prefix":"TwitchOnly","twitch":true,"minimumSubTier":0}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	return catalogue
}

func TestRosterKeepsOnlyTheChannel(t *testing.T) {
	roster := NewRoster(dggjams, nil)
	roster.Apply(frame(t, `NAMES {"users":[
		{"nick":"Anpan","features":["subscriber","flair13"],"watching":{"platform":"kick","id":"dggJams"}},
		{"nick":"Other","features":[],"watching":{"platform":"kick","id":"destiny"}},
		{"nick":"Nobody","features":[],"watching":null}
	]}`), time.Now())

	list := roster.List()
	if nicks(list) != "Anpan" {
		t.Fatalf("got %q", nicks(list))
	}
	// flair13 comes after subscriber in flairs.css, so it wins.
	if list[0].Flair == nil || *list[0].Flair != "flair13" {
		t.Fatalf("flair %v", list[0].Flair)
	}
}

func TestRosterFollowsJoinsQuitsAndSwitches(t *testing.T) {
	roster := NewRoster(dggjams, nil)
	now := time.Now()
	roster.Apply(frame(t, `JOIN {"nick":"A","watching":{"platform":"kick","id":"dggjams"}}`), now)
	roster.Apply(frame(t, `JOIN {"nick":"B","watching":{"platform":"kick","id":"dggjams"}}`), now)
	roster.Apply(frame(t, `QUIT {"nick":"a"}`), now)
	if nicks(roster.List()) != "B" {
		t.Fatalf("after quit: %q", nicks(roster.List()))
	}
	// Speaking from another embed is how a switch is seen.
	roster.Apply(frame(t, `MSG {"nick":"B","data":"hi","watching":{"platform":"kick","id":"destiny"}}`), now)
	if roster.Len() != 0 {
		t.Fatalf("after switch: %q", nicks(roster.List()))
	}
	roster.Apply(frame(t, `USERSDELTA {"users":[{"nick":"C","watching":{"platform":"kick","id":"dggjams"}}],"removed":[]}`), now)
	roster.Apply(frame(t, `USERSDELTA {"users":[],"removed":[{"nick":"C"}]}`), now)
	if roster.Len() != 0 {
		t.Fatalf("after delta: %q", nicks(roster.List()))
	}
}

func TestRosterOrdersBySpeakingAndKeepsItAcrossNames(t *testing.T) {
	catalogue := testCatalogue(t)
	roster := NewRoster(dggjams, catalogue.LastEmote)
	start := time.Now()
	join := `{"nick":"%s","watching":{"platform":"kick","id":"dggjams"}}`
	for _, nick := range []string{"Quiet", "First", "Second"} {
		roster.Apply(frame(t, "JOIN "+strings.Replace(join, "%s", nick, 1)), start)
	}
	roster.Apply(frame(t, `MSG {"nick":"First","data":"catJAM","watching":{"platform":"kick","id":"dggjams"}}`), start.Add(time.Second))
	roster.Apply(frame(t, `MSG {"nick":"Second","data":"words","watching":{"platform":"kick","id":"dggjams"}}`), start.Add(2*time.Second))
	if nicks(roster.List()) != "Second,First,Quiet" {
		t.Fatalf("order %q", nicks(roster.List()))
	}

	// A reconnect re-sends NAMES. Who spoke, and with what, is kept.
	roster.Apply(frame(t, `NAMES {"users":[
		{"nick":"First","watching":{"platform":"kick","id":"dggjams"}},
		{"nick":"Second","watching":{"platform":"kick","id":"dggjams"}}
	]}`), start.Add(3*time.Second))
	list := roster.List()
	if nicks(list) != "Second,First" {
		t.Fatalf("after names %q", nicks(list))
	}
	if list[1].LastEmote == nil || *list[1].LastEmote != "catJAM" {
		t.Fatalf("emote lost: %v", list[1].LastEmote)
	}
	// Plain words leave the last emote alone.
	if list[0].LastEmote != nil {
		t.Fatalf("words became an emote: %v", *list[0].LastEmote)
	}
}

func TestRosterReplaysHistory(t *testing.T) {
	roster := NewRoster(dggjams, nil)
	roster.Apply(frame(t, `HISTORY ["MSG {\"nick\":\"Old\",\"data\":\"hey\",\"timestamp\":1700000000000,\"watching\":{\"platform\":\"kick\",\"id\":\"dggjams\"}}"]`), time.Now())
	list := roster.List()
	if nicks(list) != "Old" || list[0].LastSpokeAt == nil || list[0].LastSpokeAt.UnixMilli() != 1700000000000 {
		t.Fatalf("history: %+v", list)
	}
}

func TestLastEmote(t *testing.T) {
	catalogue := testCatalogue(t)
	cases := []struct {
		text string
		tier int
		want string
	}{
		{"this song catJAM yes", 0, "catJAM"},
		{"catJAM pepeJAM", 0, "pepeJAM"},
		{"catJAMing", 0, ""},
		{"catjam", 0, ""},
		{"cat", 0, "cat"},
		{"PepoTurkey", 1, ""},
		{"PepoTurkey", 2, "PepoTurkey"},
		{"catJAM PepoTurkey", 1, "catJAM"},
		{"TwitchOnly", 5, ""},
		{"", 0, ""},
	}
	for _, c := range cases {
		if got := catalogue.LastEmote(c.text, c.tier); got != c.want {
			t.Errorf("LastEmote(%q, %d) = %q, want %q", c.text, c.tier, got, c.want)
		}
	}
}

func TestCatalogueVersionFollowsTheManifest(t *testing.T) {
	a, _ := ParseCatalogue([]byte(`[{"prefix":"A"}]`))
	b, _ := ParseCatalogue([]byte(`[{"prefix":"A"},{"prefix":"Dancer"}]`))
	if a.Version == b.Version || len(a.Version) != 12 {
		t.Fatalf("versions %q %q", a.Version, b.Version)
	}
}

func TestRetryDelayStaysInItsWindow(t *testing.T) {
	if got := retryDelay(0, func() float64 { return 0 }); got != retryMin {
		t.Fatalf("floor %v", got)
	}
	if got := retryDelay(40, func() float64 { return 0.999999 }); got > retryWindowCap {
		t.Fatalf("cap %v", got)
	}
}
