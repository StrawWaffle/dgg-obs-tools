// Package settings holds what the user chose on the setup page, saved beside
// the user's other application settings so it survives restarts and updates.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
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
}

// Settings is everything the setup page saves.
type Settings struct {
	// The embed whose watchers are drawn, or nil before one is chosen.
	Channel *dgg.Channel `json:"channel"`
	Overlay Overlay      `json:"overlay"`
}

// Default is a fresh install.
func Default() Settings {
	return Settings{Overlay: Overlay{
		Show: "speakers", Window: 10, Max: 12, Layout: "float", Names: "under",
		Enter: "fade", Motion: "drift", Color: "flair", Speed: 100, Size: 100, Roam: 100, Inset: 4,
	}}
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

// Validate checks every field and lowercases the channel id.
func (s *Settings) Validate() error {
	if s.Channel != nil {
		s.Channel.ID = strings.ToLower(strings.TrimSpace(s.Channel.ID))
		if err := oneOf("platform", s.Channel.Platform, Platforms); err != nil {
			return err
		}
		if s.Channel.ID == "" || len(s.Channel.ID) > 120 {
			return errors.New("channel must be a name of up to 120 characters")
		}
	}
	o := s.Overlay
	return errors.Join(
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
