// Package kick reads Kick chat anonymously, the way kick.com's own page does
// for a viewer who is not signed in.
//
// Kick moved its chat from Pusher to Centrifugo in 2026, and the page now asks
// which to use rather than having a key built in. So a connection is three
// steps, all without an account:
//
//  1. GET kick.com/api/v2/channels/<slug> for the channel's chatroom id.
//  2. POST web.kick.com/api/v1/realtime/auth/connection with a random client
//     id, which answers with a short-lived connection token.
//  3. Open Centrifugo, connect with that token, and subscribe to
//     `chatrooms.<id>.v2`, the same channel name Pusher used.
//
// None of this is a published API, and it can change again without notice.
package kick

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	mathrand "math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
)

const (
	channelURL    = "https://kick.com/api/v2/channels/"
	connectionURL = "https://web.kick.com/api/v1/realtime/auth/connection"
	realtimeURL   = "https://web.kick.com/api/v1/realtime/channels/%d/chat/connection"
	// Used when the realtime lookup fails; it is what that lookup answered in November 2026.
	defaultSocket = "wss://realtime.us-east-1.platform.kick.com/connection/websocket"

	// kick.com sits behind Cloudflare, which turns away clients that do not
	// look like a browser.
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"

	stableAfter = 5 * time.Second
	// The server pings every 25 seconds; twice that without a frame means the
	// connection is gone.
	silenceLimit = 60 * time.Second
	readLimit    = 1 << 20
)

// Message is one chat message, reduced to what an overlay draws.
type Message struct {
	// The channel slug it was sent in, lowercase.
	Channel string
	Nick    string
	// The name colour Kick gives the sender, or "".
	Color string
	// The last emote in the message: its name, and its picture.
	Emote    string
	EmoteURL string
	At       time.Time
}

// Client follows the chat of some Kick channels over one connection.
type Client struct {
	// Channel slugs, lowercase.
	Channels  []string
	OnMessage func(Message)

	http     http.Client
	mu       sync.Mutex
	channels map[string]ids
}

// ids are the two numbers Kick keys a channel by: its own, and its chat's.
type ids struct {
	channel  int64
	chatroom int64
}

// Run connects and keeps reconnecting until ctx ends.
func (c *Client) Run(ctx context.Context) {
	c.http.Timeout = 15 * time.Second
	attempts := 0
	for ctx.Err() == nil {
		attempts++
		openedAt, err := c.connectOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("kick chat: %v", err)
		}
		if !openedAt.IsZero() && time.Since(openedAt) >= stableAfter {
			attempts = 0
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(chat.RetryDelay(attempts, mathrand.Float64)):
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) (time.Time, error) {
	rooms := map[int64]string{}
	var firstChannel int64
	for _, slug := range c.Channels {
		found, err := c.lookup(ctx, slug)
		if err != nil {
			// One channel that cannot be found should not cost the others their chat.
			log.Printf("kick chat: %s: %v", slug, err)
			continue
		}
		rooms[found.chatroom] = slug
		if firstChannel == 0 {
			firstChannel = found.channel
		}
	}
	if len(rooms) == 0 {
		return time.Time{}, errors.New("no channel could be found")
	}

	clientID := newClientID()
	socketURL := c.socketURL(ctx, firstChannel, clientID)
	token, err := c.connectionToken(ctx, clientID)
	if err != nil {
		return time.Time{}, fmt.Errorf("connection token: %w", err)
	}

	dialCtx, cancelDial := context.WithTimeout(ctx, 10*time.Second)
	conn, _, err := websocket.Dial(dialCtx, socketURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"User-Agent": []string{userAgent}},
	})
	cancelDial()
	if err != nil {
		return time.Time{}, err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(readLimit)
	openedAt := time.Now()

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	write := func(value any) error {
		data, _ := json.Marshal(value)
		writeCtx, cancelWrite := context.WithTimeout(connCtx, 5*time.Second)
		defer cancelWrite()
		return conn.Write(writeCtx, websocket.MessageText, data)
	}

	if err := write(map[string]any{"id": 1, "connect": map[string]any{"name": "js", "token": token}}); err != nil {
		return openedAt, err
	}
	next := 2
	for id := range rooms {
		if err := write(map[string]any{"id": next, "subscribe": map[string]any{"channel": fmt.Sprintf("chatrooms.%d.v2", id)}}); err != nil {
			return openedAt, err
		}
		next++
	}

	// The connection token expires (about 45 minutes). Reconnecting a minute
	// early with a fresh one is simpler than Centrifugo's refresh command, and
	// a viewer misses a second of chat at most.
	expires := time.NewTimer(40 * time.Minute)
	defer expires.Stop()
	lastFrame := time.Now()
	var lastFrameMu sync.Mutex
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-connCtx.Done():
				return
			case <-expires.C:
				_ = conn.Close(websocket.StatusNormalClosure, "token expiring")
				return
			case <-ticker.C:
				lastFrameMu.Lock()
				quiet := time.Since(lastFrame) > silenceLimit
				lastFrameMu.Unlock()
				if quiet {
					_ = conn.CloseNow()
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.Read(connCtx)
		if err != nil {
			return openedAt, err
		}
		lastFrameMu.Lock()
		lastFrame = time.Now()
		lastFrameMu.Unlock()
		// Centrifugo can batch several replies into one frame, a line each.
		for _, line := range bytes.Split(data, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			// An empty object is the server's ping, answered with one.
			if string(line) == "{}" {
				_ = write(map[string]any{})
				continue
			}
			if err := c.handle(line, rooms); err != nil {
				return openedAt, err
			}
		}
	}
}

type reply struct {
	ID    int `json:"id"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Push *struct {
		Channel string `json:"channel"`
		Pub     *struct {
			Data struct {
				Event string `json:"event"`
				Data  string `json:"data"`
			} `json:"data"`
		} `json:"pub"`
	} `json:"push"`
}

type chatMessage struct {
	ChatroomID int64  `json:"chatroom_id"`
	Content    string `json:"content"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
	Sender     struct {
		Username string `json:"username"`
		Identity struct {
			Color string `json:"color"`
		} `json:"identity"`
	} `json:"sender"`
}

func (c *Client) handle(line []byte, rooms map[int64]string) error {
	var r reply
	if json.Unmarshal(line, &r) != nil {
		return nil
	}
	if r.Error != nil {
		return fmt.Errorf("centrifugo %d: %s", r.Error.Code, r.Error.Message)
	}
	if r.Push == nil || r.Push.Pub == nil || r.Push.Pub.Data.Event != `App\Events\ChatMessageEvent` {
		return nil
	}
	var m chatMessage
	if json.Unmarshal([]byte(r.Push.Pub.Data.Data), &m) != nil {
		return nil
	}
	if message, ok := toMessage(m, rooms); ok && c.OnMessage != nil {
		c.OnMessage(message)
	}
	return nil
}

var emoteTag = regexp.MustCompile(`\[emote:(\d+):([^\]\s]+)\]`)

func toMessage(m chatMessage, rooms map[int64]string) (Message, bool) {
	slug, ok := rooms[m.ChatroomID]
	if !ok || m.Sender.Username == "" || chat.IsBot(m.Sender.Username) {
		return Message{}, false
	}
	at, err := time.Parse(time.RFC3339, m.CreatedAt)
	if err != nil {
		at = time.Now()
	}
	message := Message{Channel: slug, Nick: m.Sender.Username, Color: safeColor(m.Sender.Identity.Color), At: at.UTC()}
	if found := emoteTag.FindAllStringSubmatch(m.Content, -1); len(found) > 0 {
		last := found[len(found)-1]
		message.Emote = last[2]
		message.EmoteURL = "https://files.kick.com/emotes/" + last[1] + "/fullsize"
	}
	return message, true
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// safeColor keeps a colour only when it is plainly one, since it is put
// straight into the overlay's styles.
func safeColor(color string) string {
	if hexColor.MatchString(color) {
		return color
	}
	return ""
}

func (c *Client) lookup(ctx context.Context, slug string) (ids, error) {
	c.mu.Lock()
	found, ok := c.channels[slug]
	c.mu.Unlock()
	if ok {
		return found, nil
	}

	var channel struct {
		ID       int64 `json:"id"`
		Chatroom struct {
			ID int64 `json:"id"`
		} `json:"chatroom"`
	}
	if err := c.getJSON(ctx, http.MethodGet, channelURL+slug, nil, &channel); err != nil {
		return ids{}, err
	}
	if channel.Chatroom.ID == 0 {
		return ids{}, errors.New("no chatroom")
	}
	found = ids{channel: channel.ID, chatroom: channel.Chatroom.ID}
	c.mu.Lock()
	if c.channels == nil {
		c.channels = map[string]ids{}
	}
	c.channels[slug] = found
	c.mu.Unlock()
	return found, nil
}

// socketURL asks Kick where its realtime server is, the way the page does. On
// any trouble it falls back to the address it answered last.
func (c *Client) socketURL(ctx context.Context, channel int64, clientID string) string {
	body := map[string]any{
		"client":       map[string]any{"id": clientID, "type": "web"},
		"capabilities": map[string]any{"accepted_providers": []any{map[string]any{"provider": "centrifugo"}}},
	}
	var answer struct {
		Data struct {
			Connections []struct {
				Provider    string `json:"provider"`
				Credentials struct {
					URL string `json:"url"`
				} `json:"credentials"`
			} `json:"connections"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, http.MethodPost, fmt.Sprintf(realtimeURL, channel), body, &answer); err == nil {
		for _, connection := range answer.Data.Connections {
			if connection.Provider == "centrifugo" && strings.HasPrefix(connection.Credentials.URL, "wss://") {
				return connection.Credentials.URL
			}
		}
	}
	return defaultSocket
}

func (c *Client) connectionToken(ctx context.Context, clientID string) (string, error) {
	var answer struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, http.MethodPost, connectionURL, map[string]string{"client_id": clientID}, &answer); err != nil {
		return "", err
	}
	if answer.Data.Token == "" {
		return "", errors.New("no token in the answer")
	}
	return answer.Data.Token, nil
}

func (c *Client) getJSON(ctx context.Context, method, url string, body, into any) error {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d", url, response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(into)
}

// newClientID is a random UUID, which is what the page uses for a viewer who
// is not signed in.
func newClientID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
