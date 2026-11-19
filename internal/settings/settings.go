// Package settings holds what the user chose on the setup page, saved beside
// the user's other application settings so it survives restarts and updates.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
	"github.com/StrawWaffle/dgg-obs-tools/internal/youtube"
)

// The platforms destiny.gg embeds, spelled the way a chatter's `watching` spells them.
var Platforms = []string{"kick", "youtube", "twitch", "angelthump"}

var (
	shows     = []string{"speakers", "all"}
	layouts   = []string{"float", "safe", "rail", "column", "sides", "climb", "bump"}
	names     = []string{"under", "beside", "off"}
	entrances = []string{"fade", "spin", "slide", "random"}
	motions   = []string{"drift", "bob", "orbit", "sway"}
	colors    = []string{"flair", "white"}
	switches  = []string{"on", "off"}

	dggChat   = []string{"all", "watching", "off"}
	shadows   = []string{"none", "soft", "strong"}
	linkStyle = []string{"full", "short"}
	removed   = []string{"hide", "blur", "keep"}
)

// Overlay is how the watchers overlay draws. The web side has the same shape
// in web/src/watchers/options.ts.
type Overlay struct {
	Show   string `json:"show"`
	Window int    `json:"window"`
	Max    int    `json:"max"`
	Layout string `json:"layout"`
	Names  string `json:"names"`
	Enter  string `json:"enter"`
	Motion string `json:"motion"`
	Color  string `json:"color"`
	Speed  int    `json:"speed"`
	Size   int    `json:"size"`
	Roam   int    `json:"roam"`
	Inset  int    `json:"inset"`
	// Whether Kick and YouTube names carry a small K or YT.
	Marks string `json:"marks"`
}

// Chat is how the combined chat page draws.
type Chat struct {
	// Which destiny.gg messages it shows: everything, only from people
	// watching one of the followed embeds, or none.
	DGG string `json:"dgg"`
	// Whether the followed Kick, YouTube and Twitch chats show.
	Kick    bool `json:"kick"`
	YouTube bool `json:"youtube"`
	Twitch  bool `json:"twitch"`
	// Percent of the standard size, text and emotes together.
	Size   int    `json:"size"`
	Shadow string `json:"shadow"`
	// The small platform icon before each name.
	Icons string `json:"icons"`
	// destiny.gg's flair badges beside names, as its chat shows them.
	Badges string `json:"badges"`
	// Whether swear words and slurs are starred out.
	Censor string `json:"censor"`
	// Phrases that highlight a message, one per entry, matched without case.
	Highlights []string `json:"highlights"`
	// Whether destiny.gg messages from people watching a followed embed stand out.
	HighlightWatching string `json:"highlightWatching"`
	// Links in full, or shortened to their site.
	Links string `json:"links"`
	// What happens to a message a moderator removes.
	Removed string `json:"removed"`
	// How many messages the page keeps.
	Lines int `json:"lines"`
}

// Settings is everything the setup page saves. The three sources are
// independent: an embed can be followed without its stream's chat, and a
// Kick or YouTube chat without any embed.
type Settings struct {
	// destiny.gg embeds, whose watchers are named by destiny.gg chat.
	Embeds []dgg.Channel `json:"embeds"`
	// Kick channels whose chat is read, by slug.
	KickChats []string `json:"kickChats"`
	// YouTube chats to read: a channel handle such as "@destiny", followed
	// from stream to stream, or one video id.
	YouTubeChats []string `json:"youtubeChats"`
	// Twitch channels whose chat is read, by login.
	TwitchChats []string `json:"twitchChats"`
	Overlay     Overlay  `json:"overlay"`
	Chat        Chat     `json:"chat"`
}

// Default is a fresh install.
func Default() Settings {
	return Settings{Embeds: []dgg.Channel{}, KickChats: []string{}, YouTubeChats: []string{}, TwitchChats: []string{}, Overlay: Overlay{
		Show: "speakers", Window: 10, Max: 12, Layout: "float", Names: "under",
		Enter: "fade", Motion: "drift", Color: "flair", Speed: 100, Size: 100, Roam: 100, Inset: 4, Marks: "on",
	}, Chat: Chat{
		DGG: "all", Kick: true, YouTube: true, Twitch: true, Size: 100, Shadow: "soft", Icons: "on",
		Badges: "on", Censor: "off",
		Highlights: []string{}, HighlightWatching: "on", Links: "short", Removed: "hide", Lines: 150,
	}}
}

// Empty reports whether nothing at all is followed.
func (s Settings) Empty() bool {
	return len(s.Embeds) == 0 && len(s.KickChats) == 0 && len(s.YouTubeChats) == 0 && len(s.TwitchChats) == 0
}

func oneOf(field, value string, allowed []string) error {
	if !slices.Contains(allowed, value) {
		return fmt.Errorf("%s must be one of %s", field, strings.Join(allowed, ", "))
	}
	return nil
}

func within(field string, value, low, high int) error {
	if value < low || value > high {
		return fmt.Errorf("%s must be between %d and %d", field, low, high)
	}
	return nil
}

var (
	youtubeHandle = regexp.MustCompile(`^@[\w.-]{3,30}$`)
	kickSlug      = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)
	twitchLogin   = regexp.MustCompile(`^[a-z0-9_]{3,25}$`)
)

// MaxPerList is how many embeds, or chats of one platform, an overlay
// follows. Enough for a co-stream or a watch party; more would only be a
// wall of faces.
const MaxPerList = 10

// youtubeEntry checks a YouTube handle or video id. A video id is case
// sensitive; a handle is not, and is kept lowercase so it compares.
func youtubeEntry(entry string) (string, error) {
	if youtube.IsVideoID(entry) {
		return entry, nil
	}
	entry = strings.ToLower(entry)
	if !youtubeHandle.MatchString(entry) {
		return "", fmt.Errorf("%q is neither a YouTube video id nor a channel handle such as @destiny", entry)
	}
	return entry, nil
}

// unique keeps the first of each, in order.
func unique[T comparable](items []T) []T {
	kept := make([]T, 0, len(items))
	for _, item := range items {
		if !slices.Contains(kept, item) {
			kept = append(kept, item)
		}
	}
	return kept
}

// Validate checks every field, lowercases names that are not case sensitive
// and drops repeats.
func (s *Settings) Validate() error {
	if len(s.Embeds) > MaxPerList || len(s.KickChats) > MaxPerList || len(s.YouTubeChats) > MaxPerList || len(s.TwitchChats) > MaxPerList {
		return fmt.Errorf("at most %d embeds, and %d chats on each platform", MaxPerList, MaxPerList)
	}

	embeds := make([]dgg.Channel, 0, len(s.Embeds))
	for _, embed := range s.Embeds {
		embed.ID = strings.TrimSpace(embed.ID)
		if err := oneOf("platform", embed.Platform, Platforms); err != nil {
			return err
		}
		if embed.ID == "" || len(embed.ID) > 120 {
			return errors.New("each embed needs a name of up to 120 characters")
		}
		if embed.Platform == "youtube" {
			// destiny.gg names a YouTube embed by its video; a handle is
			// looked up for whatever that channel has live.
			id, err := youtubeEntry(embed.ID)
			if err != nil {
				return err
			}
			embed.ID = id
		} else {
			embed.ID = strings.ToLower(embed.ID)
		}
		embeds = append(embeds, embed)
	}
	s.Embeds = unique(embeds)

	kicks := make([]string, 0, len(s.KickChats))
	for _, slug := range s.KickChats {
		slug = strings.ToLower(strings.TrimSpace(slug))
		if !kickSlug.MatchString(slug) {
			return fmt.Errorf("%q is not a Kick channel name", slug)
		}
		kicks = append(kicks, slug)
	}
	s.KickChats = unique(kicks)

	videos := make([]string, 0, len(s.YouTubeChats))
	for _, entry := range s.YouTubeChats {
		entry, err := youtubeEntry(strings.TrimSpace(entry))
		if err != nil {
			return err
		}
		videos = append(videos, entry)
	}
	s.YouTubeChats = unique(videos)

	logins := make([]string, 0, len(s.TwitchChats))
	for _, login := range s.TwitchChats {
		login = strings.ToLower(strings.TrimSpace(login))
		if !twitchLogin.MatchString(login) {
			return fmt.Errorf("%q is not a Twitch channel name", login)
		}
		logins = append(logins, login)
	}
	s.TwitchChats = unique(logins)

	c := &s.Chat
	if len(c.Highlights) > 50 {
		return errors.New("at most 50 highlighted phrases")
	}
	highlights := make([]string, 0, len(c.Highlights))
	for _, phrase := range c.Highlights {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		if len(phrase) > 60 {
			return errors.New("a highlighted phrase can be at most 60 characters")
		}
		highlights = append(highlights, phrase)
	}
	c.Highlights = unique(highlights)

	o := s.Overlay
	return errors.Join(
		oneOf("chat dgg", c.DGG, dggChat),
		within("chat size", c.Size, 50, 300),
		oneOf("chat shadow", c.Shadow, shadows),
		oneOf("chat icons", c.Icons, switches),
		oneOf("chat badges", c.Badges, switches),
		oneOf("chat censor", c.Censor, switches),
		oneOf("chat highlightWatching", c.HighlightWatching, switches),
		oneOf("chat links", c.Links, linkStyle),
		oneOf("chat removed", c.Removed, removed),
		within("chat lines", c.Lines, 20, 300),
		oneOf("show", o.Show, shows),
		within("window", o.Window, 1, 1440),
		within("max", o.Max, 1, 100),
		oneOf("layout", o.Layout, layouts),
		oneOf("names", o.Names, names),
		oneOf("enter", o.Enter, entrances),
		oneOf("motion", o.Motion, motions),
		oneOf("color", o.Color, colors),
		within("speed", o.Speed, 10, 400),
		within("size", o.Size, 25, 400),
		within("roam", o.Roam, 0, 300),
		within("inset", o.Inset, 0, 25),
		oneOf("marks", o.Marks, switches),
	)
}

// Path is where the settings live: the user config directory, so a portable
// executable can be moved or replaced by an update without losing them.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dgg-obs-tools", "settings.json"), nil
}

// Load reads the saved settings. A missing or unreadable file is a fresh
// install rather than an error: there is nothing in it worth refusing to start over.
func Load(path string) Settings {
	loaded := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return loaded
	}
	if json.Unmarshal(data, &loaded) != nil || loaded.Validate() != nil {
		return Default()
	}
	return loaded
}

// Save writes the settings through a temporary file, so a crash mid-write
// leaves the previous settings rather than half of the new ones.
func Save(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
