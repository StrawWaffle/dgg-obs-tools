// Package twitch reads Twitch chat anonymously over its IRC WebSocket.
//
// Twitch lets anybody read chat without an account by logging in as
// `justinfan` followed by digits. That nick can join channels and read, and
// nothing else, which is all this needs. Unlike Kick's and YouTube's, this is
// a documented interface.
package twitch

import (
	"context"
	"fmt"
	"log"
	mathrand "math/rand/v2"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
)

const (
	socketURL = "wss://irc-ws.chat.twitch.tv:443"
	// Twitch pings about every five minutes; more than that without a line
	// means the connection is gone.
	silenceLimit = 6 * time.Minute
	stableAfter  = 5 * time.Second
)

// Message is one chat message, reduced to what the overlay and chat draw.
type Message struct {
	// The channel it was sent in, lowercase, without the #.
	Channel string
	ID      string
	Nick    string
	// The name colour the sender chose, or "".
	Color  string
	Action bool
	Parts  []chat.Part
	// The last emote in the message: its name, and its picture.
	Emote    string
	EmoteURL string
	At       time.Time
}

// Client follows the chat of some Twitch channels over one connection.
type Client struct {
	// Channel logins, lowercase.
	Channels  []string
	OnMessage func(Message)
	OnRemove  func(chat.Removal)
}

// Run connects and keeps reconnecting until ctx ends.
func (c *Client) Run(ctx context.Context) {
	attempts := 0
	for ctx.Err() == nil {
		attempts++
		openedAt, err := c.connectOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("twitch chat: %v", err)
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
	dialCtx, cancelDial := context.WithTimeout(ctx, 10*time.Second)
	conn, _, err := websocket.Dial(dialCtx, socketURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"User-Agent": []string{"dgg-obs-tools (+https://github.com/StrawWaffle/dgg-obs-tools)"}},
	})
	cancelDial()
	if err != nil {
		return time.Time{}, err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	openedAt := time.Now()

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	send := func(line string) error {
		writeCtx, cancelWrite := context.WithTimeout(connCtx, 5*time.Second)
		defer cancelWrite()
		return conn.Write(writeCtx, websocket.MessageText, []byte(line+"\r\n"))
	}

	channels := make([]string, len(c.Channels))
	for i, channel := range c.Channels {
		channels[i] = "#" + channel
	}
	for _, line := range []string{
		"CAP REQ :twitch.tv/tags twitch.tv/commands",
		"PASS SCHMOOPIIE",
		fmt.Sprintf("NICK justinfan%d", 10000+mathrand.IntN(89999)),
		"JOIN " + strings.Join(channels, ","),
	} {
		if err := send(line); err != nil {
			return openedAt, err
		}
	}

	var lastLine atomic.Int64
	lastLine.Store(time.Now().UnixNano())
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-connCtx.Done():
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, lastLine.Load())) > silenceLimit {
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
		lastLine.Store(time.Now().UnixNano())
		for _, line := range strings.Split(string(data), "\r\n") {
			if line == "" {
				continue
			}
			if rest, ok := strings.CutPrefix(line, "PING "); ok {
				if err := send("PONG " + rest); err != nil {
					return openedAt, err
				}
				continue
			}
			if strings.Contains(line, " RECONNECT") {
				// Twitch is about to restart that server and asks clients to move.
				return openedAt, nil
			}
			c.handle(line)
		}
	}
}

func (c *Client) handle(line string) {
	parsed, ok := parseLine(line)
	if !ok {
		return
	}
	switch parsed.command {
	case "PRIVMSG":
		if message, ok := toMessage(parsed); ok && c.OnMessage != nil {
			c.OnMessage(message)
		}
	case "CLEARCHAT", "CLEARMSG":
		if removal, ok := toRemoval(parsed); ok && c.OnRemove != nil {
			c.OnRemove(removal)
		}
	}
}

// line is one IRC line: `@tags :prefix COMMAND #channel :text`.
type line struct {
	tags    map[string]string
	login   string
	command string
	channel string
	text    string
}

func parseLine(raw string) (line, bool) {
	var parsed line
	if rest, ok := strings.CutPrefix(raw, "@"); ok {
		tags, after, found := strings.Cut(rest, " ")
		if !found {
			return line{}, false
		}
		parsed.tags = map[string]string{}
		for _, tag := range strings.Split(tags, ";") {
			key, value, _ := strings.Cut(tag, "=")
			parsed.tags[key] = unescapeTag(value)
		}
		raw = after
	}
	if rest, ok := strings.CutPrefix(raw, ":"); ok {
		prefix, after, found := strings.Cut(rest, " ")
		if !found {
			return line{}, false
		}
		parsed.login, _, _ = strings.Cut(prefix, "!")
		raw = after
	}
	head, text, _ := strings.Cut(raw, " :")
	fields := strings.Fields(head)
	if len(fields) == 0 {
		return line{}, false
	}
	parsed.command = fields[0]
	if len(fields) > 1 {
		parsed.channel = strings.TrimPrefix(fields[1], "#")
	}
	parsed.text = text
	return parsed, true
}

var tagEscapes = strings.NewReplacer(`\s`, " ", `\:`, ";", `\\`, `\`, `\r`, "\r", `\n`, "\n")

func unescapeTag(value string) string {
	return tagEscapes.Replace(value)
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func toMessage(l line) (Message, bool) {
	nick := l.tags["display-name"]
	if nick == "" {
		nick = l.login
	}
	if nick == "" || chat.IsBot(nick) {
		return Message{}, false
	}
	text := l.text
	action := false
	if inner, ok := strings.CutPrefix(text, "\x01ACTION "); ok {
		text = strings.TrimSuffix(inner, "\x01")
		action = true
	}
	message := Message{
		Channel: l.channel,
		ID:      "twitch-" + l.tags["id"],
		Nick:    nick,
		Action:  action,
		At:      time.Now().UTC(),
	}
	if hexColor.MatchString(l.tags["color"]) {
		message.Color = l.tags["color"]
	}
	if ms, err := strconv.ParseInt(l.tags["tmi-sent-ts"], 10, 64); err == nil {
		message.At = time.UnixMilli(ms).UTC()
	}
	message.Parts = emoteParts(text, l.tags["emotes"])
	for _, part := range message.Parts {
		if part.URL != "" {
			message.Emote, message.EmoteURL = part.Emote, part.URL
		}
	}
	return message, true
}

// emoteParts splits a message at the emotes Twitch lists in its `emotes`
// tag, as `id:start-end,start-end/id:start-end`, counted in characters.
func emoteParts(text, tag string) []chat.Part {
	type span struct {
		start, end int
		id         string
	}
	var spans []span
	for _, entry := range strings.Split(tag, "/") {
		id, ranges, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		for _, r := range strings.Split(ranges, ",") {
			from, to, ok := strings.Cut(r, "-")
			start, errStart := strconv.Atoi(from)
			end, errEnd := strconv.Atoi(to)
			if ok && errStart == nil && errEnd == nil && start <= end {
				spans = append(spans, span{start, end, id})
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	runes := []rune(text)
	var parts []chat.Part
	last := 0
	for _, s := range spans {
		if s.start < last || s.end >= len(runes) {
			continue
		}
		parts = append(parts, chat.TextParts(string(runes[last:s.start]))...)
		parts = append(parts, chat.Part{
			Emote: string(runes[s.start : s.end+1]),
			URL:   "https://static-cdn.jtvnw.net/emoticons/v2/" + s.id + "/default/dark/2.0",
		})
		last = s.end + 1
	}
	return append(parts, chat.TextParts(string(runes[last:]))...)
}

// toRemoval reads a moderator clearing one message (CLEARMSG) or somebody's
// messages with a ban or timeout (CLEARCHAT with a name). A CLEARCHAT with no
// name clears the whole chat, which is left alone here.
func toRemoval(l line) (chat.Removal, bool) {
	if l.command == "CLEARMSG" && l.tags["target-msg-id"] != "" {
		return chat.Removal{Platform: chat.Twitch, IDs: []string{"twitch-" + l.tags["target-msg-id"]}}, true
	}
	if l.command == "CLEARCHAT" && l.text != "" {
		return chat.Removal{Platform: chat.Twitch, Nick: l.text}, true
	}
	return chat.Removal{}, false
}
