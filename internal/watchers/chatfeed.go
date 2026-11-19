package watchers

import (
	"context"
	"slices"
	"strings"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
)

// historyLimit is how many recent messages are kept for a chat page that
// opens, and so the most any page can show.
const historyLimit = 300

// chatFrame takes what a chat page needs from a destiny.gg frame: messages,
// the backlog chat sends on connect, and mutes and bans. Called with t.mu held.
func (t *Tracker) chatFrame(frame dgg.Frame) {
	frames := []dgg.Frame{frame}
	if backlog := dgg.HistoryFrames(frame); backlog != nil {
		frames = backlog
	}
	for _, f := range frames {
		if message, ok := dgg.ChatMessage(f, t.catalogue, t.flairs, t.isWatching); ok {
			t.add(message)
		}
		if removal, ok := dgg.ChatRemoval(f); ok {
			t.applyRemoval(removal)
		}
	}
}

// isWatching says whether a destiny.gg chatter's embed is a followed one.
func (t *Tracker) isWatching(channel dgg.Channel) bool {
	for _, followed := range t.dggChannels() {
		if followed.Platform == channel.Platform && strings.EqualFold(followed.ID, channel.ID) {
			return true
		}
	}
	return false
}

// say records a Kick, YouTube or Twitch message.
func (t *Tracker) say(ctx context.Context, message chat.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx.Err() == nil {
		t.add(message)
	}
}

// add keeps a message and passes it to every open chat page. A message
// already held is skipped: destiny.gg's backlog on every reconnect, and
// YouTube's first poll, repeat what was said before. Called with t.mu held.
func (t *Tracker) add(message chat.Message) {
	if message.ID == "" || slices.ContainsFunc(t.history, func(held chat.Message) bool { return held.ID == message.ID }) {
		return
	}
	if len(t.history) >= historyLimit {
		t.history = slices.Delete(t.history, 0, len(t.history)-historyLimit+1)
	}
	t.history = append(t.history, message)
	t.publish(&t.chats, Event{Type: "message", Data: message})
}

// remove applies a moderator's removal from Kick or YouTube.
func (t *Tracker) remove(ctx context.Context, removal chat.Removal) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx.Err() == nil {
		t.applyRemoval(removal)
	}
}

// Removed is what a chat page is told: the ids of messages taken down.
type Removed struct {
	IDs []string `json:"ids"`
}

// applyRemoval marks the messages a removal names, by id or by who sent them,
// and tells the open chat pages which. Called with t.mu held.
func (t *Tracker) applyRemoval(removal chat.Removal) {
	var ids []string
	for i := range t.history {
		message := &t.history[i]
		if message.Platform != removal.Platform {
			continue
		}
		byID := slices.Contains(removal.IDs, message.ID)
		byNick := removal.Nick != "" && strings.EqualFold(message.Nick, removal.Nick)
		byAuthor := removal.AuthorID != "" && message.AuthorID == removal.AuthorID
		if byID || byNick || byAuthor {
			message.Removed = true
			ids = append(ids, message.ID)
		}
	}
	if len(ids) > 0 {
		t.publish(&t.chats, Event{Type: "remove", Data: Removed{IDs: ids}})
	}
}

// forgetUnfollowedMessages drops Kick, YouTube and Twitch messages from chats no
// longer followed. destiny.gg is one chat, so its messages stay.
// Called with t.mu held.
func (t *Tracker) forgetUnfollowedMessages() {
	t.history = slices.DeleteFunc(t.history, func(message chat.Message) bool {
		switch message.Platform {
		case chat.Kick:
			return !slices.Contains(t.settings.KickChats, message.Source)
		case chat.YouTube:
			return !slices.Contains(t.settings.YouTubeChats, message.Source)
		case chat.Twitch:
			return !slices.Contains(t.settings.TwitchChats, message.Source)
		}
		return false
	})
}
