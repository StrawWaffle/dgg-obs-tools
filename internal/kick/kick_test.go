package kick

import (
	"encoding/json"
	"testing"
)

// A ChatMessageEvent as Kick's Centrifugo socket delivered it in November 2026,
// trimmed to the fields read here.
const recorded = `{"chatroom_id":1764849,"content":"[emote:37227:LULW] no way [emote:5748004:collectiblesGoldenNAHH]","type":"message","created_at":"2026-11-19T08:55:54+00:00","sender":{"username":"GrumSkider","identity":{"color":"#FF55B3"}}}`

func TestToMessageTakesTheLastEmote(t *testing.T) {
	var m chatMessage
	if err := json.Unmarshal([]byte(recorded), &m); err != nil {
		t.Fatal(err)
	}
	message, ok := toMessage(m, map[int64]string{1764849: "destiny"})
	if !ok {
		t.Fatal("message dropped")
	}
	if message.Channel != "destiny" || message.Nick != "GrumSkider" || message.Color != "#FF55B3" {
		t.Fatalf("got %+v", message)
	}
	if message.Emote != "collectiblesGoldenNAHH" || message.EmoteURL != "https://files.kick.com/emotes/5748004/fullsize" {
		t.Fatalf("emote %q %q", message.Emote, message.EmoteURL)
	}
}

func TestToMessageSkipsOtherRoomsAndBots(t *testing.T) {
	var m chatMessage
	_ = json.Unmarshal([]byte(recorded), &m)
	if _, ok := toMessage(m, map[int64]string{1: "other"}); ok {
		t.Fatal("a message for another chatroom was kept")
	}
	m.Sender.Username = "BotRix"
	if _, ok := toMessage(m, map[int64]string{1764849: "destiny"}); ok {
		t.Fatal("a bot was kept")
	}
}

func TestSafeColor(t *testing.T) {
	for color, want := range map[string]string{"#FF55B3": "#FF55B3", "red": "", "#fff;x:y": "", "": ""} {
		if got := safeColor(color); got != want {
			t.Errorf("safeColor(%q) = %q", color, got)
		}
	}
}
