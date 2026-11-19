package dgg

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// ChatSocketURL is destiny.gg's chat. An anonymous connection answers `ME null`,
// then HISTORY, then NAMES (the whole roster), then the live event stream.
const ChatSocketURL = "wss://chat.destiny.gg/ws"

// Channel is one embed, named the way a chatter's `watching` names it.
type Channel struct {
	Platform string `json:"platform"`
	// Lowercase. Chat reports ids lowercase while bigscreen links are written
	// `#kick/dggJams`, so every comparison folds case.
	ID string `json:"id"`
}

// Frame is one event in the chat server's `EVENT {json}` line format.
type Frame struct {
	Event string
	Data  json.RawMessage
}

// ParseFrame splits a line into its event name and payload. A payload that is
// not JSON is kept as nil data, so an unexpected frame is ignored rather than
// fatal.
func ParseFrame(raw string) (Frame, bool) {
	event, payload, _ := strings.Cut(raw, " ")
	if event == "" {
		return Frame{}, false
	}
	frame := Frame{Event: strings.ToUpper(event)}
	if json.Valid([]byte(payload)) {
		frame.Data = json.RawMessage(payload)
	}
	return frame, true
}

type chatUser struct {
	Nick         string   `json:"nick"`
	Features     []string `json:"features"`
	Subscription *struct {
		Tier int `json:"tier"`
	} `json:"subscription"`
	Watching *struct {
		Platform string `json:"platform"`
		ID       string `json:"id"`
	} `json:"watching"`
	// Milliseconds, on MSG and JOIN.
	Timestamp *int64 `json:"timestamp"`
	// The message text, on MSG.
	Data *string `json:"data"`
}

func (u chatUser) subTier() int {
	if u.Subscription == nil {
		return 0
	}
	return u.Subscription.Tier
}

func (u chatUser) watches(channel Channel) bool {
	return u.Watching != nil &&
		u.Watching.Platform == channel.Platform &&
		strings.ToLower(u.Watching.ID) == channel.ID
}

// Watcher is one person in chat with the channel's embed open.
type Watcher struct {
	Nick string `json:"nick"`
	// The flair their name is coloured by, or null.
	Flair *string `json:"flair"`
	// When they last spoke, which is also when `watching` was last confirmed:
	// switching embeds is not broadcast, so it is only seen when they talk.
	LastSpokeAt *time.Time `json:"lastSpokeAt"`
	// The last emote they used that their account can use, or null.
	LastEmote *string `json:"lastEmote"`
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// EmoteReader finds the emote in a message, or "".
type EmoteReader func(text string, subTier int) string

// Roster is who is watching one channel, kept from the chat event stream.
//
// NAMES seeds it on every connect; JOIN, MSG and UPDATEUSER set one person;
// QUIT and USERSDELTA remove. When somebody last spoke survives a NAMES rebuild,
// because a reconnect does not mean nobody has talked.
type Roster struct {
	channel   Channel
	readEmote EmoteReader
	watchers  map[string]Watcher
}

// NewRoster starts an empty roster for one channel.
func NewRoster(channel Channel, readEmote EmoteReader) *Roster {
	return &Roster{channel: channel, readEmote: readEmote, watchers: map[string]Watcher{}}
}

// Len is how many people are on the channel.
func (r *Roster) Len() int { return len(r.watchers) }

// List puts whoever spoke most recently first, then everyone else by name.
func (r *Roster) List() []Watcher {
	list := make([]Watcher, 0, len(r.watchers))
	for _, watcher := range r.watchers {
		list = append(list, watcher)
	}
	sort.Slice(list, func(i, j int) bool {
		left, right := list[i].LastSpokeAt, list[j].LastSpokeAt
		switch {
		case left != nil && right != nil && !left.Equal(*right):
			return left.After(*right)
		case left != nil && right == nil:
			return true
		case left == nil && right != nil:
			return false
		}
		return list[i].Nick < list[j].Nick
	})
	return list
}

// Apply folds one frame into the roster.
func (r *Roster) Apply(frame Frame, now time.Time) {
	if frame.Data == nil {
		return
	}
	switch frame.Event {
	case "NAMES":
		r.applyNames(frame.Data)
	case "HISTORY":
		r.applyHistory(frame.Data, now)
	case "JOIN", "UPDATEUSER":
		r.applyUser(frame.Data, nil)
	case "MSG":
		r.applyUser(frame.Data, &now)
	case "QUIT":
		r.applyQuit(frame.Data)
	case "USERSDELTA":
		r.applyDelta(frame.Data)
	}
}

func (r *Roster) applyNames(data json.RawMessage) {
	var names struct {
		Users []chatUser `json:"users"`
	}
	if json.Unmarshal(data, &names) != nil {
		return
	}
	rebuilt := make(map[string]Watcher, len(r.watchers))
	for _, user := range names.Users {
		if user.Nick == "" || !user.watches(r.channel) {
			continue
		}
		key := strings.ToLower(user.Nick)
		known := r.watchers[key]
		rebuilt[key] = Watcher{
			Nick:        user.Nick,
			Flair:       optional(ResolveFlair(user.Features)),
			LastSpokeAt: known.LastSpokeAt,
			LastEmote:   known.LastEmote,
		}
	}
	r.watchers = rebuilt
}

// applyHistory replays the backlog chat sends on connect, so an overlay knows
// who has been talking the moment it opens.
func (r *Roster) applyHistory(data json.RawMessage, now time.Time) {
	var lines []string
	if json.Unmarshal(data, &lines) != nil {
		return
	}
	for _, line := range lines {
		if frame, ok := ParseFrame(line); ok && frame.Event != "HISTORY" {
			r.Apply(frame, now)
		}
	}
}

func (r *Roster) applyUser(data json.RawMessage, spokeAt *time.Time) {
	var user chatUser
	if json.Unmarshal(data, &user) != nil {
		return
	}
	r.setUser(user, spokeAt)
}

func (r *Roster) setUser(user chatUser, spokeAt *time.Time) {
	if user.Nick == "" {
		return
	}
	key := strings.ToLower(user.Nick)
	if !user.watches(r.channel) {
		delete(r.watchers, key)
		return
	}

	known := r.watchers[key]
	spoken := known.LastSpokeAt
	if spokeAt != nil {
		at := *spokeAt
		if user.Timestamp != nil {
			at = time.UnixMilli(*user.Timestamp)
		}
		at = at.UTC()
		spoken = &at
	}
	// Only a message carries text, and only an emote in one replaces the last:
	// a message of plain words leaves it as it was.
	emote := known.LastEmote
	if user.Data != nil && r.readEmote != nil {
		if found := r.readEmote(*user.Data, user.subTier()); found != "" {
			emote = &found
		}
	}
	r.watchers[key] = Watcher{
		Nick:        user.Nick,
		Flair:       optional(ResolveFlair(user.Features)),
		LastSpokeAt: spoken,
		LastEmote:   emote,
	}
}

func (r *Roster) applyQuit(data json.RawMessage) {
	var quit struct {
		Nick string `json:"nick"`
	}
	if json.Unmarshal(data, &quit) != nil {
		return
	}
	delete(r.watchers, strings.ToLower(quit.Nick))
}

func (r *Roster) applyDelta(data json.RawMessage) {
	var delta struct {
		Users   []chatUser `json:"users"`
		Removed []struct {
			Nick string `json:"nick"`
		} `json:"removed"`
	}
	if json.Unmarshal(data, &delta) != nil {
		return
	}
	for _, user := range delta.Users {
		r.setUser(user, nil)
	}
	for _, gone := range delta.Removed {
		delete(r.watchers, strings.ToLower(gone.Nick))
	}
}
