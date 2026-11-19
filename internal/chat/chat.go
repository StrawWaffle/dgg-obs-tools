// Package chat holds what destiny.gg, Kick and YouTube chat have in common:
// the person an overlay draws, and how a dropped connection is retried.
package chat

import (
	"regexp"
	"strings"
	"time"
)

// Platforms a watcher can come from.
const (
	DGG     = "dgg"
	Kick    = "kick"
	YouTube = "youtube"
	Twitch  = "twitch"
)

// Watcher is one person an overlay can draw. On destiny.gg that is somebody
// with a followed embed open; on Kick and YouTube, somebody talking in a
// followed stream's own chat, which says as much.
type Watcher struct {
	Nick     string `json:"nick"`
	Platform string `json:"platform"`
	// The destiny.gg flair the name is coloured by, or null.
	Flair *string `json:"flair"`
	// A name colour the platform gives directly (Kick), or null.
	Color *string `json:"color"`
	// When they last said something. On destiny.gg that is also when their
	// `watching` was last confirmed, since switching embeds is not broadcast.
	LastSpokeAt *time.Time `json:"lastSpokeAt"`
	// The last emote they used. A destiny.gg emote is drawn from destiny.gg's
	// stylesheet by this name; any other has EmoteURL.
	LastEmote *string `json:"lastEmote"`
	// The picture of a Kick or YouTube emote, or null for destiny.gg's own.
	EmoteURL *string `json:"emoteUrl"`
}

// Key tells people apart. The same name on two platforms is two people.
func (w Watcher) Key() string {
	return w.Platform + "/" + strings.ToLower(w.Nick)
}

// Optional turns "" into null.
func Optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// PictureURL accepts a plain https address and nothing else, because it ends
// up inside a CSS url() on the overlay.
var pictureURL = regexp.MustCompile(`^https://[^\s"'()\\]+$`)

// SafePicture returns url when it is safe to draw, and "" otherwise.
func SafePicture(url string) string {
	if pictureURL.MatchString(url) {
		return url
	}
	return ""
}

// Bots on Kick and YouTube, skipped by name: they talk constantly and are not
// anybody watching.
var bots = map[string]bool{
	"botrix": true, "kickbot": true, "nightbot": true, "streamelements": true,
	"fossabot": true, "moobot": true, "streamlabs": true, "sery_bot": true,
}

// IsBot reports whether a Kick or YouTube name is a known bot.
func IsBot(nick string) bool {
	return bots[strings.ToLower(strings.TrimPrefix(nick, "@"))]
}

const (
	retryWindow    = 3 * time.Second
	retryWindowCap = 60 * time.Second
	retryMin       = 500 * time.Millisecond
)

// RetryDelay is chat-gui's backoff, used for every connection here: full
// jitter over a window that doubles with each failed attempt. The jitter
// matters as much as the backoff, because a mass disconnect releases every
// client at the same moment.
func RetryDelay(attempts int, random func() float64) time.Duration {
	window := retryWindowCap
	if attempts < 6 {
		window = min(retryWindowCap, retryWindow*time.Duration(1<<attempts))
	}
	return retryMin + time.Duration(random()*float64(window-retryMin))
}
