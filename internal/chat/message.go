package chat

import (
	"regexp"
	"strings"
	"time"
)

// Message is one chat message from any platform, already split into what the
// chat page draws.
type Message struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	// Where it was said: "destiny.gg", or the Kick channel or YouTube entry.
	Source string    `json:"source"`
	Nick   string    `json:"nick"`
	Flair  *string   `json:"flair"`
	Color  *string   `json:"color"`
	At     time.Time `json:"at"`
	Parts  []Part    `json:"parts"`
	// The destiny.gg flair badges chat shows beside the name.
	Badges []Badge `json:"badges,omitempty"`
	// A destiny.gg /me message, drawn in italics.
	Action bool `json:"action,omitempty"`
	// A destiny.gg message starting with ">", drawn green as chat does.
	Green bool `json:"green,omitempty"`
	// On destiny.gg: whether the sender has one of the followed embeds open.
	Watching bool `json:"watching,omitempty"`
	// Removed by a moderator after it was sent.
	Removed bool `json:"removed,omitempty"`

	// The YouTube channel id of the sender, so a ban on them can find their
	// messages. Never sent to a page.
	AuthorID string `json:"-"`
}

// Badge is a small picture beside a name, such as a destiny.gg flair.
type Badge struct {
	Label  string `json:"label"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Part is one piece of a message: plain text, an emote, or a link.
type Part struct {
	Text string `json:"text,omitempty"`
	// An emote's name. Without URL it is a destiny.gg emote, drawn from
	// destiny.gg's stylesheet; with one it is drawn from that picture.
	Emote string `json:"emote,omitempty"`
	URL   string `json:"url,omitempty"`
	Link  string `json:"link,omitempty"`
}

// Removal is a moderator taking messages down: some by id, or everything by
// one person on a platform.
type Removal struct {
	Platform string
	IDs      []string
	Nick     string
	AuthorID string
}

var link = regexp.MustCompile(`https?://[^\s<>"]+`)

// TextParts splits plain text into text and links. Emotes are each
// platform's business, so they arrive as their own parts.
func TextParts(text string) []Part {
	var parts []Part
	last := 0
	for _, found := range link.FindAllStringIndex(text, -1) {
		if found[0] > last {
			parts = append(parts, Part{Text: text[last:found[0]]})
		}
		url := strings.TrimRight(text[found[0]:found[1]], ".,!?)")
		parts = append(parts, Part{Link: url})
		last = found[0] + len(url)
	}
	if last < len(text) {
		parts = append(parts, Part{Text: text[last:]})
	}
	return parts
}

// WordParts splits text at whitespace and turns each word that isEmote names
// into an emote, which is how destiny.gg chat finds its emotes; the rest goes
// through TextParts, and neighbouring text is joined back up.
func WordParts(text string, isEmote func(word string) bool) []Part {
	var parts []Part
	var pending strings.Builder
	flush := func() {
		if pending.Len() > 0 {
			parts = append(parts, TextParts(pending.String())...)
			pending.Reset()
		}
	}
	start := -1
	for i, r := range text + " " {
		space := r == ' ' || r == '\t' || r == '\n'
		if !space {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			word := text[start:i]
			if isEmote(word) {
				flush()
				parts = append(parts, Part{Emote: word})
			} else {
				pending.WriteString(word)
			}
			start = -1
		}
		if i < len(text) {
			pending.WriteRune(r)
		}
	}
	flush()
	return parts
}
