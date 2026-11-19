package youtube

import (
	"encoding/json"
	"testing"
)

// One addChatItemAction as get_live_chat answered it in November 2026,
// trimmed to the fields read here.
const recorded = `{"addChatItemAction":{"item":{"liveChatTextMessageRenderer":{
	"authorName":{"simpleText":"@FyterianTV"},"timestampUsec":"1763542554000000",
	"message":{"runs":[{"text":"lol "},{"emoji":{"shortcuts":[":grinning_squinting_face:"],
	"image":{"thumbnails":[{"url":"https://fonts.gstatic.com/s/e/notoemoji/15.1/1f606/72.png"}]}}}]}}}}}`

func TestToMessage(t *testing.T) {
	var a action
	if err := json.Unmarshal([]byte(recorded), &a); err != nil {
		t.Fatal(err)
	}
	message, ok := toMessage(a, "@destiny", "P7Xv5DdqL0k")
	if !ok {
		t.Fatal("message dropped")
	}
	if message.Nick != "FyterianTV" || message.Emote != "grinning_squinting_face" ||
		message.EmoteURL != "https://fonts.gstatic.com/s/e/notoemoji/15.1/1f606/72.png" ||
		message.At.UnixMicro() != 1763542554000000 {
		t.Fatalf("got %+v", message)
	}
}

func TestIsVideoID(t *testing.T) {
	for entry, want := range map[string]bool{"P7Xv5DdqL0k": true, "@destiny": false, "destiny": false} {
		if IsVideoID(entry) != want {
			t.Errorf("IsVideoID(%q)", entry)
		}
	}
}
