// Package watchers keeps track of who is watching the followed embeds and
// hands that to every open overlay.
//
// Three independent sources feed it. destiny.gg chat names everyone with a
// followed embed open, talking or not. Kick and YouTube chat name the people
// talking in a followed stream's own chat, who are watching it too; they are
// kept from their messages, since neither platform lists who is in a chat.
package watchers

import (
	"context"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
	"github.com/StrawWaffle/dgg-obs-tools/internal/kick"
	"github.com/StrawWaffle/dgg-obs-tools/internal/settings"
	"github.com/StrawWaffle/dgg-obs-tools/internal/youtube"
)

const (
	// The connections are held while an overlay or the setup page is open,
	// and for this long after the last one closes, so a scene switch or an OBS
	// reload does not cost a reconnect. Nobody looking means nothing to draw,
	// and no reason to sit on anybody's chat.
	idleGrace = 2 * time.Minute

	// Chat runs at a few events a second. Overlays are sent at most one
	// snapshot a second, which is as often as their own clock ticks anyway.
	pushEvery = time.Second

	catalogueEvery = time.Hour

	// Enough for any overlay, and it keeps each push small on a busy channel.
	watcherLimit = 100

	// Kick and YouTube never say when somebody leaves, so somebody who has
	// not talked for this long is taken to have gone. The speaking window, if
	// longer, keeps them for as long as it asks.
	speakerRetention = 30 * time.Minute
)

// Snapshot is what an overlay draws from. Every field of it is already public
// in the chats it comes from.
type Snapshot struct {
	Embeds       []dgg.Channel `json:"embeds"`
	KickChats    []string      `json:"kickChats"`
	YouTubeChats []string      `json:"youtubeChats"`
	// For each followed YouTube channel handle, the live stream found on it, or "".
	LiveVideos   map[string]string `json:"liveVideos"`
	ChatCount    int               `json:"chatCount"`
	Watchers     []chat.Watcher    `json:"watchers"`
	EmoteVersion *string           `json:"emoteVersion"`
	Connected    bool              `json:"connected"`
}

// Event is one message to an open page.
type Event struct {
	Type string // "snapshot" or "settings"
	Data any
}

// Tracker holds the connections and the people they report.
type Tracker struct {
	settingsPath string
	client       *http.Client

	mu        sync.Mutex
	settings  settings.Settings
	catalogue *dgg.Catalogue

	// destiny.gg chat. New embeds need a new connection, because chat only
	// sends its roster on connect.
	roster     *dgg.Roster
	socket     *dgg.Socket
	stopSocket context.CancelFunc

	// Kick and YouTube. YouTube channels are followed from stream to stream,
	// and live maps each followed channel to the stream it is on now.
	stopOthers context.CancelFunc
	live       map[string]string
	// Who has talked on Kick or YouTube, by chat.Watcher.Key.
	speakers map[string]chat.Watcher

	subscribers map[chan Event]struct{}
	lastSeen    time.Time
	dirty       bool
}

// New loads the saved settings. Nothing connects until Run.
func New(settingsPath string) *Tracker {
	t := &Tracker{
		settingsPath: settingsPath,
		client:       &http.Client{},
		settings:     settings.Load(settingsPath),
		live:         map[string]string{},
		speakers:     map[string]chat.Watcher{},
		subscribers:  map[chan Event]struct{}{},
	}
	t.roster = t.newRoster()
	return t
}

// dggChannels are the embeds destiny.gg chat is asked about. A YouTube
// channel counts once a live stream has been found on it, because destiny.gg
// names a YouTube embed by its video.
func (t *Tracker) dggChannels() []dgg.Channel {
	var channels []dgg.Channel
	for _, channel := range t.settings.Embeds {
		if channel.Platform == chat.YouTube && !youtube.IsVideoID(channel.ID) {
			video := t.live[channel.ID]
			if video == "" {
				continue
			}
			channel.ID = video
		}
		channels = append(channels, channel)
	}
	return channels
}

func (t *Tracker) newRoster() *dgg.Roster {
	channels := t.dggChannels()
	if len(channels) == 0 {
		return nil
	}
	return dgg.NewRoster(channels, func(text string, tier int) string {
		// Called with t.mu held, from onFrame.
		return t.catalogue.LastEmote(text, tier)
	})
}

// Run keeps the emote list fresh, opens and closes the connections as pages
// come and go, and pushes snapshots until ctx ends.
func (t *Tracker) Run(ctx context.Context) {
	go t.refreshCatalogue(ctx)

	ticker := time.NewTicker(pushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.mu.Lock()
			t.closeAll()
			t.mu.Unlock()
			return
		case <-ticker.C:
			t.mu.Lock()
			t.gate(ctx)
			t.forgetQuietSpeakers()
			if t.dirty {
				t.dirty = false
				t.publish(Event{Type: "snapshot", Data: t.snapshot()})
			}
			t.mu.Unlock()
		}
	}
}

func (t *Tracker) refreshCatalogue(ctx context.Context) {
	for {
		catalogue, err := dgg.FetchCatalogue(ctx, t.client)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("emote list: %v", err)
		} else {
			t.mu.Lock()
			t.catalogue = catalogue
			t.dirty = true
			t.mu.Unlock()
		}
		wait := catalogueEvery
		if err != nil {
			wait = time.Minute
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// gate opens the connections when there is something to follow and somebody
// to draw for, and closes them once there has been nobody for idleGrace.
func (t *Tracker) gate(ctx context.Context) {
	if len(t.subscribers) > 0 {
		t.lastSeen = time.Now()
	}
	wanted := !t.settings.Empty() && time.Since(t.lastSeen) < idleGrace
	if !wanted {
		if t.socket != nil || t.stopOthers != nil {
			t.closeAll()
			t.dirty = true
		}
		return
	}
	if t.socket == nil && len(t.settings.Embeds) > 0 {
		t.openDGG(ctx)
	}
	if t.stopOthers == nil {
		t.openOthers(ctx)
	}
}

func (t *Tracker) openDGG(ctx context.Context) {
	socketCtx, cancel := context.WithCancel(ctx)
	socket := &dgg.Socket{URL: dgg.ChatSocketURL}
	socket.OnFrame = func(raw string) { t.onFrame(socketCtx, socket, raw) }
	t.socket, t.stopSocket = socket, cancel
	go socket.Run(socketCtx)
}

// openOthers starts Kick chat and a source for each YouTube entry. A YouTube
// handle among the embeds is looked up even when its chat is not read,
// because destiny.gg names that embed by the live video.
func (t *Tracker) openOthers(ctx context.Context) {
	othersCtx, cancel := context.WithCancel(ctx)
	t.stopOthers = cancel

	readChat := map[string]bool{}
	for _, entry := range t.settings.YouTubeChats {
		readChat[entry] = true
	}
	entries := slices.Clone(t.settings.YouTubeChats)
	for _, embed := range t.settings.Embeds {
		if embed.Platform == chat.YouTube && !youtube.IsVideoID(embed.ID) && !slices.Contains(entries, embed.ID) {
			entries = append(entries, embed.ID)
		}
	}
	for _, entry := range entries {
		source := &youtube.Source{
			Entry:      entry,
			LookupOnly: !readChat[entry],
			OnLive:     func(video string) { t.onLive(othersCtx, entry, video) },
			OnMessage:  func(m youtube.Message) { t.onYouTube(othersCtx, m) },
		}
		go source.Run(othersCtx)
	}

	if len(t.settings.KickChats) > 0 {
		client := &kick.Client{Channels: t.settings.KickChats, OnMessage: func(m kick.Message) { t.onKick(othersCtx, m) }}
		go client.Run(othersCtx)
	}
}

func (t *Tracker) closeAll() {
	if t.stopSocket != nil {
		t.stopSocket()
	}
	t.socket, t.stopSocket = nil, nil
	if t.stopOthers != nil {
		t.stopOthers()
	}
	t.stopOthers = nil
	t.roster = t.newRoster()
}

func (t *Tracker) onFrame(ctx context.Context, socket *dgg.Socket, raw string) {
	// Rare, and separate from the protocol pings the socket answers itself.
	if payload, ok := strings.CutPrefix(raw, "PING "); ok {
		go socket.Send(ctx, "PONG "+payload)
		return
	}
	frame, ok := dgg.ParseFrame(raw)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.socket != socket || t.roster == nil {
		return
	}
	t.roster.Apply(frame, time.Now())
	t.dirty = true
}

// onLive records the stream found on a YouTube channel. destiny.gg names that
// embed by its video, so a new stream means asking destiny.gg chat again.
func (t *Tracker) onLive(ctx context.Context, entry, video string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx.Err() != nil || t.live[entry] == video {
		return
	}
	t.live[entry] = video
	t.roster = t.newRoster()
	if t.socket != nil {
		t.socket.Reconnect()
	}
	t.dirty = true
}

func (t *Tracker) onKick(ctx context.Context, m kick.Message) {
	t.speak(ctx, chat.Watcher{
		Nick:      m.Nick,
		Platform:  chat.Kick,
		Color:     chat.Optional(m.Color),
		LastEmote: chat.Optional(m.Emote),
		EmoteURL:  chat.Optional(m.EmoteURL),
	}, m.At)
}

func (t *Tracker) onYouTube(ctx context.Context, m youtube.Message) {
	t.speak(ctx, chat.Watcher{
		Nick:      m.Nick,
		Platform:  chat.YouTube,
		LastEmote: chat.Optional(m.Emote),
		EmoteURL:  chat.Optional(m.EmoteURL),
	}, m.At)
}

// speak records somebody talking on Kick or YouTube. As on destiny.gg, a
// message without an emote leaves the last one they used.
func (t *Tracker) speak(ctx context.Context, w chat.Watcher, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	w.LastSpokeAt = &at
	if known, ok := t.speakers[w.Key()]; ok {
		if known.LastSpokeAt != nil && known.LastSpokeAt.After(at) {
			// The backlog YouTube sends first can arrive after a newer message.
			return
		}
		if w.LastEmote == nil {
			w.LastEmote, w.EmoteURL = known.LastEmote, known.EmoteURL
		}
	}
	t.speakers[w.Key()] = w
	t.dirty = true
}

func (t *Tracker) forgetQuietSpeakers() {
	keep := max(speakerRetention, time.Duration(t.settings.Overlay.Window)*time.Minute)
	for key, speaker := range t.speakers {
		if time.Since(*speaker.LastSpokeAt) > keep {
			delete(t.speakers, key)
			t.dirty = true
		}
	}
}

func (t *Tracker) snapshot() Snapshot {
	snapshot := Snapshot{
		Embeds:       t.settings.Embeds,
		KickChats:    t.settings.KickChats,
		YouTubeChats: t.settings.YouTubeChats,
		LiveVideos:   map[string]string{},
		Watchers:     []chat.Watcher{},
	}
	for _, embed := range t.settings.Embeds {
		if embed.Platform == chat.YouTube && !youtube.IsVideoID(embed.ID) {
			snapshot.LiveVideos[embed.ID] = t.live[embed.ID]
		}
	}
	for _, entry := range t.settings.YouTubeChats {
		if !youtube.IsVideoID(entry) {
			snapshot.LiveVideos[entry] = t.live[entry]
		}
	}
	if t.catalogue != nil {
		snapshot.EmoteVersion = &t.catalogue.Version
	}
	if t.socket != nil {
		snapshot.Connected = t.socket.State().Connected
	}

	var everyone []chat.Watcher
	if t.roster != nil {
		everyone = t.roster.List()
	}
	for _, speaker := range t.speakers {
		everyone = append(everyone, speaker)
	}
	snapshot.ChatCount = len(everyone)
	sortBySpeaking(everyone)
	snapshot.Watchers = everyone[:min(len(everyone), watcherLimit)]
	return snapshot
}

// sortBySpeaking puts whoever spoke most recently first, then everyone else
// by name, so a capped list keeps the people an overlay is likeliest to draw.
func sortBySpeaking(watchers []chat.Watcher) {
	sort.SliceStable(watchers, func(i, j int) bool {
		left, right := watchers[i].LastSpokeAt, watchers[j].LastSpokeAt
		switch {
		case left != nil && right != nil && !left.Equal(*right):
			return left.After(*right)
		case left != nil && right == nil:
			return true
		case left == nil && right != nil:
			return false
		}
		return watchers[i].Key() < watchers[j].Key()
	})
}

func (t *Tracker) publish(event Event) {
	for subscriber := range t.subscribers {
		select {
		case subscriber <- event:
		default:
			// A page that has fallen this far behind gets the next one instead.
		}
	}
}

// Subscribe registers an open page. It is sent the settings and a snapshot
// straight away, then every change. Call the returned function when it closes.
func (t *Tracker) Subscribe() (<-chan Event, func()) {
	events := make(chan Event, 4)
	t.mu.Lock()
	t.subscribers[events] = struct{}{}
	t.lastSeen = time.Now()
	events <- Event{Type: "settings", Data: t.settings}
	events <- Event{Type: "snapshot", Data: t.snapshot()}
	t.mu.Unlock()

	return events, func() {
		t.mu.Lock()
		delete(t.subscribers, events)
		t.lastSeen = time.Now()
		t.mu.Unlock()
	}
}

// Settings returns what is saved.
func (t *Tracker) Settings() settings.Settings {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.settings
}

// SetSettings validates, saves, and applies new settings. Open overlays redraw
// with them straight away.
func (t *Tracker) SetSettings(next settings.Settings) error {
	if err := next.Validate(); err != nil {
		return err
	}
	if err := settings.Save(t.settingsPath, next); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	previous := t.settings
	t.settings = next
	if !slices.Equal(previous.Embeds, next.Embeds) ||
		!slices.Equal(previous.KickChats, next.KickChats) ||
		!slices.Equal(previous.YouTubeChats, next.YouTubeChats) {
		// Everything follows the new list from scratch; the next tick reopens
		// what is wanted. Kick and YouTube speakers from embeds no longer
		// followed go with it.
		t.closeAll()
		t.live = map[string]string{}
		t.speakers = map[string]chat.Watcher{}
		t.roster = t.newRoster()
	}
	t.publish(Event{Type: "settings", Data: t.settings})
	t.publish(Event{Type: "snapshot", Data: t.snapshot()})
	return nil
}
