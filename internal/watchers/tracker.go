// Package watchers keeps the roster for the chosen channel and hands it to
// every open overlay.
package watchers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
	"github.com/StrawWaffle/dgg-obs-tools/internal/settings"
)

const (
	// The chat connection is held while an overlay or the setup page is open,
	// and for this long after the last one closes, so a scene switch or an OBS
	// reload does not cost a reconnect. Nobody looking means nothing to draw,
	// and no reason to sit on destiny.gg's chat.
	idleGrace = 2 * time.Minute

	// Chat runs at a couple of events a second. Overlays are sent at most one
	// snapshot a second, which is as often as their own clock ticks anyway.
	pushEvery = time.Second

	catalogueEvery = time.Hour

	// Enough for any overlay, and it keeps each push small on a busy channel.
	watcherLimit = 100
)

// Snapshot is what an overlay draws from. Every field of it is already public
// in destiny.gg chat.
type Snapshot struct {
	Channel      *dgg.Channel  `json:"channel"`
	ChatCount    int           `json:"chatCount"`
	Watchers     []dgg.Watcher `json:"watchers"`
	EmoteVersion *string       `json:"emoteVersion"`
	Connected    bool          `json:"connected"`
}

// Event is one message to an open page.
type Event struct {
	Type string // "snapshot" or "settings"
	Data any
}

// Tracker is the one chat connection and roster for the app.
type Tracker struct {
	settingsPath string
	client       *http.Client

	mu          sync.Mutex
	settings    settings.Settings
	roster      *dgg.Roster
	catalogue   *dgg.Catalogue
	socket      *dgg.Socket
	stopSocket  context.CancelFunc
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
		subscribers:  map[chan Event]struct{}{},
	}
	t.roster = t.newRoster()
	return t
}

func (t *Tracker) newRoster() *dgg.Roster {
	if t.settings.Channel == nil {
		return nil
	}
	return dgg.NewRoster(*t.settings.Channel, func(text string, tier int) string {
		// Called with t.mu held, from onFrame.
		return t.catalogue.LastEmote(text, tier)
	})
}

// Run keeps the emote list fresh, opens and closes the chat connection as
// pages come and go, and pushes snapshots until ctx ends.
func (t *Tracker) Run(ctx context.Context) {
	go t.refreshCatalogue(ctx)

	ticker := time.NewTicker(pushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.mu.Lock()
			t.closeChat()
			t.mu.Unlock()
			return
		case <-ticker.C:
			t.mu.Lock()
			t.gate(ctx)
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

// gate opens the chat connection when there is a channel and somebody to
// draw for, and closes it once there has been nobody for idleGrace.
func (t *Tracker) gate(ctx context.Context) {
	if len(t.subscribers) > 0 {
		t.lastSeen = time.Now()
	}
	wanted := t.settings.Channel != nil && time.Since(t.lastSeen) < idleGrace
	switch {
	case wanted && t.socket == nil:
		socketCtx, cancel := context.WithCancel(ctx)
		socket := &dgg.Socket{URL: dgg.ChatSocketURL}
		socket.OnFrame = func(raw string) { t.onFrame(socketCtx, socket, raw) }
		t.socket, t.stopSocket = socket, cancel
		go socket.Run(socketCtx)
	case !wanted && t.socket != nil:
		t.closeChat()
		t.dirty = true
	}
}

func (t *Tracker) closeChat() {
	if t.stopSocket != nil {
		t.stopSocket()
	}
	t.socket, t.stopSocket = nil, nil
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

func (t *Tracker) snapshot() Snapshot {
	snapshot := Snapshot{Channel: t.settings.Channel, Watchers: []dgg.Watcher{}}
	if t.catalogue != nil {
		snapshot.EmoteVersion = &t.catalogue.Version
	}
	if t.socket != nil {
		snapshot.Connected = t.socket.State().Connected
	}
	if t.roster != nil {
		snapshot.ChatCount = t.roster.Len()
		list := t.roster.List()
		snapshot.Watchers = list[:min(len(list), watcherLimit)]
	}
	return snapshot
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
	previous := t.settings.Channel
	t.settings = next
	if !sameChannel(previous, next.Channel) {
		t.roster = t.newRoster()
		// The roster only arrives on connect, so a new channel needs a new connection.
		if t.socket != nil {
			if next.Channel == nil {
				t.closeChat()
			} else {
				t.socket.Reconnect()
			}
		}
	}
	t.publish(Event{Type: "settings", Data: t.settings})
	t.publish(Event{Type: "snapshot", Data: t.snapshot()})
	return nil
}

func sameChannel(a, b *dgg.Channel) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
