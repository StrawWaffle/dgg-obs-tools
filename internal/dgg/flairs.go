package dgg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
)

// FlairsURL is destiny.gg's list of flairs: each one's name, label, whether
// chat shows its badge, and the badge's picture.
const FlairsURL = "https://r2cdn.destiny.gg/flairs/flairs.json"

// Flairs maps a flair's name to the badge chat shows for it. Flairs chat
// hides (most colour-only ones) are left out.
type Flairs map[string]flairBadge

type flairBadge struct {
	badge    chat.Badge
	priority int
}

// ParseFlairs reads the flair list.
func ParseFlairs(body []byte) (Flairs, error) {
	var entries []struct {
		Name     string `json:"name"`
		Label    string `json:"label"`
		Hidden   bool   `json:"hidden"`
		Priority int    `json:"priority"`
		Image    []struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"image"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	flairs := Flairs{}
	for _, entry := range entries {
		if entry.Hidden || len(entry.Image) == 0 || chat.SafePicture(entry.Image[0].URL) == "" {
			continue
		}
		image := entry.Image[0]
		flairs[entry.Name] = flairBadge{
			badge:    chat.Badge{Label: entry.Label, URL: image.URL, Width: image.Width, Height: image.Height},
			priority: entry.Priority,
		}
	}
	return flairs, nil
}

// FetchFlairs downloads the current flair list.
func FetchFlairs(ctx context.Context, client *http.Client) (Flairs, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, FlairsURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", UserAgent)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flair list answered %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return ParseFlairs(body)
}

// Badges are the badges chat shows beside somebody with these features, in
// chat's order: by priority, lowest first.
func (f Flairs) Badges(features []string) []chat.Badge {
	var held []flairBadge
	for _, feature := range features {
		if badge, ok := f[feature]; ok {
			held = append(held, badge)
		}
	}
	sort.SliceStable(held, func(i, j int) bool { return held[i].priority < held[j].priority })
	badges := make([]chat.Badge, len(held))
	for i, badge := range held {
		badges[i] = badge.badge
	}
	return badges
}
