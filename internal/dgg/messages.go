package dgg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
)

// ChatMessage turns a MSG frame into a chat message. watching says whether a
// chatter's embed is one of the followed ones.
//
// destiny.gg messages carry no id, so one is made from who said what when.
// That also lets the backlog chat sends on every connect be told apart from
// messages already seen.
func ChatMessage(frame Frame, catalogue *Catalogue, flairs Flairs, watching func(Channel) bool) (chat.Message, bool) {
	if frame.Event != "MSG" || frame.Data == nil {
		return chat.Message{}, false
	}
	var user chatUser
	if json.Unmarshal(frame.Data, &user) != nil || user.Nick == "" || user.Data == nil {
		return chat.Message{}, false
	}

	at := time.Now()
	if user.Timestamp != nil {
		at = time.UnixMilli(*user.Timestamp)
	}
	text := *user.Data
	message := chat.Message{
		ID:       messageID(user.Nick, at, text),
		Platform: chat.DGG,
		Source:   "destiny.gg",
		Nick:     user.Nick,
		Flair:    chat.Optional(ResolveFlair(user.Features)),
		Badges:   flairs.Badges(user.Features),
		At:       at.UTC(),
	}
	if rest, ok := strings.CutPrefix(text, "/me "); ok {
		message.Action = true
		text = rest
	}
	message.Green = strings.HasPrefix(text, ">")
	if user.Watching != nil && watching != nil {
		message.Watching = watching(Channel{Platform: user.Watching.Platform, ID: strings.ToLower(user.Watching.ID)})
	}
	tier := user.subTier()
	message.Parts = chat.WordParts(text, func(word string) bool { return catalogue.usable(word, tier) })
	return message, true
}

func messageID(nick string, at time.Time, text string) string {
	sum := sha256.Sum256([]byte(nick + "\x00" + strconv.FormatInt(at.UnixMilli(), 10) + "\x00" + text))
	return "dgg-" + hex.EncodeToString(sum[:8])
}

// ChatRemoval reads a MUTE or BAN: a moderator taking somebody out of chat,
// whose messages go with them.
func ChatRemoval(frame Frame) (chat.Removal, bool) {
	if (frame.Event != "MUTE" && frame.Event != "BAN") || frame.Data == nil {
		return chat.Removal{}, false
	}
	var event struct {
		Data string `json:"data"`
	}
	if json.Unmarshal(frame.Data, &event) != nil || event.Data == "" {
		return chat.Removal{}, false
	}
	return chat.Removal{Platform: chat.DGG, Nick: event.Data}, true
}

// HistoryFrames unpacks the backlog chat sends on connect.
func HistoryFrames(frame Frame) []Frame {
	if frame.Event != "HISTORY" || frame.Data == nil {
		return nil
	}
	var lines []string
	if json.Unmarshal(frame.Data, &lines) != nil {
		return nil
	}
	frames := make([]Frame, 0, len(lines))
	for _, line := range lines {
		if parsed, ok := ParseFrame(line); ok {
			frames = append(frames, parsed)
		}
	}
	return frames
}
