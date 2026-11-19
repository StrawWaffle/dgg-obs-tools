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
	if len(message.Parts) != 2 || message.Parts[0].Text != "lol " || message.Parts[1].URL == "" {
		t.Fatalf("parts %+v", message.Parts)
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

func TestToRemoval(t *testing.T) {
	var one, everything action
	_ = json.Unmarshal([]byte(`{"removeChatItemAction":{"targetItemId":"abc"}}`), &one)
	_ = json.Unmarshal([]byte(`{"markChatItemsByAuthorAsDeletedAction":{"externalChannelId":"UC123"}}`), &everything)
	if removal, ok := toRemoval(one); !ok || removal.IDs[0] != "yt-abc" {
		t.Fatalf("one %+v", removal)
	}
	if removal, ok := toRemoval(everything); !ok || removal.AuthorID != "UC123" {
		t.Fatalf("everything %+v", removal)
	}
}
