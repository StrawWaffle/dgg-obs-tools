package twitch

import "testing"

func TestToMessage(t *testing.T) {
	l, ok := parseLine(`@badge-info=;color=#FF4500;display-name=Some\sOne;emotes=25:0-4,12-16;id=abc-123;tmi-sent-ts=1700000000000 :someone!someone@someone.tmi.twitch.tv PRIVMSG #xqc :Kappa hello Kappa`)
	if !ok {
		t.Fatal("not parsed")
	}
	message, ok := toMessage(l)
	if !ok {
		t.Fatal("dropped")
	}
	if message.Channel != "xqc" || message.Nick != "Some One" || message.Color != "#FF4500" || message.ID != "twitch-abc-123" {
		t.Fatalf("got %+v", message)
	}
	if len(message.Parts) != 3 || message.Parts[0].Emote != "Kappa" || message.Parts[1].Text != " hello " || message.Parts[2].Emote != "Kappa" {
		t.Fatalf("parts %+v", message.Parts)
	}
	if message.Parts[0].URL != "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0" {
		t.Fatalf("url %q", message.Parts[0].URL)
	}
}

func TestToMessageCountsEmotePositionsInCharacters(t *testing.T) {
	// "é" is two bytes but one character, as Twitch counts.
	l, _ := parseLine("@display-name=a;emotes=25:2-6;id=x :a!a@a PRIVMSG #c :é Kappa")
	message, _ := toMessage(l)
	if len(message.Parts) != 2 || message.Parts[1].Emote != "Kappa" {
		t.Fatalf("parts %+v", message.Parts)
	}
}

func TestToMessageReadsActions(t *testing.T) {
	l, _ := parseLine("@display-name=a;id=x :a!a@a PRIVMSG #c :\x01ACTION waves\x01")
	message, _ := toMessage(l)
	if !message.Action || message.Parts[0].Text != "waves" {
		t.Fatalf("got %+v", message)
	}
}

func TestToRemoval(t *testing.T) {
	ban, _ := parseLine("@ban-duration=600;room-id=1 :tmi.twitch.tv CLEARCHAT #c :troll")
	if removal, ok := toRemoval(ban); !ok || removal.Nick != "troll" {
		t.Fatalf("ban %+v", removal)
	}
	one, _ := parseLine("@login=troll;target-msg-id=abc :tmi.twitch.tv CLEARMSG #c :bad words")
	if removal, ok := toRemoval(one); !ok || removal.IDs[0] != "twitch-abc" {
		t.Fatalf("one %+v", removal)
	}
	whole, _ := parseLine("@room-id=1 :tmi.twitch.tv CLEARCHAT #c")
	if _, ok := toRemoval(whole); ok {
		t.Fatal("a cleared chat was read as one person's ban")
	}
}
