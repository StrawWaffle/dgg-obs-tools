package dgg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ManifestURL is the live emote list. destiny.gg names r2cdn in the chat page's
// `data-cdn` attribute; cdn.destiny.gg is an older copy that lags behind, and
// seasonal sets (Halloween, Christmas) show up here first.
const ManifestURL = "https://r2cdn.destiny.gg/emotes/emotes.json"

// Catalogue is the destiny.gg emote list, read for one thing: which emote
// somebody just used.
type Catalogue struct {
	// Prefix to the subscriber tier it needs.
	tiers map[string]int
	// Version fingerprints the manifest. The CDN serves its stylesheet with no
	// Cache-Control, so an OBS browser source keeps whatever copy it fetched,
	// across restarts, and draws a new emote as nothing. The overlay puts this
	// on the stylesheet URL so a changed catalogue is a new URL.
	Version string
}

type manifestEntry struct {
	Prefix         string `json:"prefix"`
	Twitch         bool   `json:"twitch"`
	MinimumSubTier int    `json:"minimumSubTier"`
}

// ParseCatalogue reads a manifest body. Twitch-only emotes are left out because
// destiny.gg chat never draws them.
func ParseCatalogue(body []byte) (*Catalogue, error) {
	var entries []manifestEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	tiers := make(map[string]int, len(entries))
	for _, entry := range entries {
		if entry.Prefix == "" || entry.Twitch {
			continue
		}
		tiers[entry.Prefix] = entry.MinimumSubTier
	}
	sum := sha256.Sum256(body)
	return &Catalogue{tiers: tiers, Version: hex.EncodeToString(sum[:])[:12]}, nil
}

// FetchCatalogue downloads the current manifest.
func FetchCatalogue(ctx context.Context, client *http.Client) (*Catalogue, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, ManifestURL, nil)
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
		return nil, fmt.Errorf("emote manifest answered %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return ParseCatalogue(body)
}

// LastEmote is the last emote in a message that its author could use.
//
// Chat draws an emote only when it is a whole word, case sensitive, with
// whitespace either side, and only when the author's tier allows it; anything
// else stays text. The last rather than the first, because it is the one they
// finished on.
func (c *Catalogue) LastEmote(text string, subTier int) string {
	if c == nil {
		return ""
	}
	found := ""
	for _, word := range strings.Fields(text) {
		if tier, ok := c.tiers[word]; ok && tier <= subTier {
			found = word
		}
	}
	return found
}
