// Package youtube reads a YouTube live stream's chat without an API key, the
// way youtube.com's pop-out chat does: it loads the chat page once for a
// continuation token, then polls get_live_chat with each token it is handed,
// waiting as long as YouTube asks between polls.
//
// None of this is a published API, and it can change without notice.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/StrawWaffle/dgg-obs-tools/internal/chat"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"
	// Without this, visitors from Europe get a consent page instead.
	consentCookie = "CONSENT=YES+1"

	// How often a channel that is not live is looked at again.
	offlineCheck = 2 * time.Minute
	// YouTube says how long to wait between polls, usually 5 to 10 seconds.
	// These bound it, in case it says something odd.
	minPoll = 2 * time.Second
	maxPoll = 20 * time.Second
)

// Message is one chat message, reduced to what an overlay draws.
type Message struct {
	// The entry it came from, as it was configured: "@handle" or a video id.
	Source string
	// The live video it was sent on.
	VideoID string
	Nick    string
	// The last emoji in the message: its name, and its picture.
	Emote    string
	EmoteURL string
	At       time.Time
}

// Source follows one entry: a channel handle such as "@destiny", whose
// current live stream is found and followed from one stream to the next, or a
// single video id.
type Source struct {
	Entry     string
	OnMessage func(Message)
	// OnLive is told the video being followed, or "" when there is none.
	OnLive func(videoID string)
	// LookupOnly finds the live video and reads no chat. destiny.gg names a
	// YouTube embed by its video, so a channel is looked up either way.
	LookupOnly bool

	http http.Client
}

var videoID = regexp.MustCompile(`^[\w-]{11}$`)

// IsVideoID tells a video id from a handle.
func IsVideoID(entry string) bool {
	return videoID.MatchString(entry)
}

// Run follows the entry until ctx ends.
func (s *Source) Run(ctx context.Context) {
	s.http.Timeout = 20 * time.Second
	for ctx.Err() == nil {
		video := s.Entry
		if !IsVideoID(video) {
			found, err := s.liveVideo(ctx, s.Entry)
			if err != nil && ctx.Err() == nil {
				log.Printf("youtube %s: %v", s.Entry, err)
			}
			video = found
		}
		if video == "" {
			s.live("")
			if !sleep(ctx, offlineCheck) {
				return
			}
			continue
		}

		s.live(video)
		if s.LookupOnly {
			if !sleep(ctx, offlineCheck) {
				return
			}
			continue
		}
		err := s.followChat(ctx, video)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("youtube %s: %v", video, err)
		}
		s.live("")
		// A stream that ended, or a chat that would not load. Either way, wait
		// before looking again, so a broken page is not asked for in a loop.
		if !sleep(ctx, offlineCheck/4) {
			return
		}
	}
}

func (s *Source) live(video string) {
	if s.OnLive != nil {
		s.OnLive(video)
	}
}

var (
	canonicalWatch = regexp.MustCompile(`<link rel="canonical" href="https://www\.youtube\.com/watch\?v=([\w-]{11})"`)
	isLiveNow      = regexp.MustCompile(`"isLive(?:Now)?":true`)
)

// liveVideo finds what a channel is streaming right now, from its /live page,
// which is the stream's own watch page while one is on. It answers "" when
// nothing is.
func (s *Source) liveVideo(ctx context.Context, handle string) (string, error) {
	path := handle
	if !strings.HasPrefix(path, "@") && !strings.HasPrefix(path, "channel/") {
		path = "@" + path
	}
	page, err := s.get(ctx, "https://www.youtube.com/"+path+"/live")
	if err != nil {
		return "", err
	}
	match := canonicalWatch.FindSubmatch(page)
	if match == nil || !isLiveNow.Match(page) {
		return "", nil
	}
	return string(match[1]), nil
}

var (
	firstContinuation = regexp.MustCompile(`"continuation":"([^"]+)"`)
	clientVersion     = regexp.MustCompile(`"clientVersion":"([\d.]+)"`)
)

func (s *Source) followChat(ctx context.Context, video string) error {
	page, err := s.get(ctx, "https://www.youtube.com/live_chat?is_popout=1&v="+video)
	if err != nil {
		return err
	}
	continuation := firstContinuation.FindSubmatch(page)
	version := clientVersion.FindSubmatch(page)
	if continuation == nil || version == nil {
		return errors.New("the chat page has no continuation; the stream may have no chat")
	}
	token, clientVer := string(continuation[1]), string(version[1])

	for {
		answer, err := s.poll(ctx, token, clientVer)
		if err != nil {
			return err
		}
		chat := answer.ContinuationContents.LiveChatContinuation
		for _, action := range chat.Actions {
			if message, ok := toMessage(action, s.Entry, video); ok && s.OnMessage != nil {
				s.OnMessage(message)
			}
		}
		next, wait := nextContinuation(chat.Continuations)
		if next == "" {
			// No more continuations means the stream is over.
			return nil
		}
		token = next
		if !sleep(ctx, min(max(wait, minPoll), maxPoll)) {
			return ctx.Err()
		}
	}
}

type continuation struct {
	Continuation string `json:"continuation"`
	TimeoutMs    int    `json:"timeoutMs"`
}

type pollAnswer struct {
	ContinuationContents struct {
		LiveChatContinuation struct {
			Continuations []map[string]continuation `json:"continuations"`
			Actions       []action                  `json:"actions"`
		} `json:"liveChatContinuation"`
	} `json:"continuationContents"`
}

type action struct {
	AddChatItemAction struct {
		Item struct {
			LiveChatTextMessageRenderer *textMessage `json:"liveChatTextMessageRenderer"`
		} `json:"item"`
	} `json:"addChatItemAction"`
}

type textMessage struct {
	AuthorName struct {
		SimpleText string `json:"simpleText"`
	} `json:"authorName"`
	TimestampUsec string `json:"timestampUsec"`
	Message       struct {
		Runs []struct {
			Text  string `json:"text"`
			Emoji *struct {
				Shortcuts []string `json:"shortcuts"`
				Image     struct {
					Thumbnails []struct {
						URL string `json:"url"`
					} `json:"thumbnails"`
				} `json:"image"`
			} `json:"emoji"`
		} `json:"runs"`
	} `json:"message"`
}

func nextContinuation(continuations []map[string]continuation) (string, time.Duration) {
	for _, entry := range continuations {
		// Live chat hands out one of several kinds; each carries a token.
		for _, value := range entry {
			if value.Continuation != "" {
				return value.Continuation, time.Duration(value.TimeoutMs) * time.Millisecond
			}
		}
	}
	return "", 0
}

func toMessage(a action, source, video string) (Message, bool) {
	text := a.AddChatItemAction.Item.LiveChatTextMessageRenderer
	if text == nil {
		return Message{}, false
	}
	nick := strings.TrimPrefix(text.AuthorName.SimpleText, "@")
	if nick == "" || chat.IsBot(nick) {
		return Message{}, false
	}
	message := Message{Source: source, VideoID: video, Nick: nick, At: time.Now().UTC()}
	if usec, err := strconv.ParseInt(text.TimestampUsec, 10, 64); err == nil {
		message.At = time.UnixMicro(usec).UTC()
	}
	for _, run := range text.Message.Runs {
		if run.Emoji == nil || len(run.Emoji.Image.Thumbnails) == 0 {
			continue
		}
		thumbnails := run.Emoji.Image.Thumbnails
		picture := thumbnails[len(thumbnails)-1].URL
		if strings.HasPrefix(picture, "//") {
			picture = "https:" + picture
		}
		if chat.SafePicture(picture) == "" {
			continue
		}
		message.EmoteURL = picture
		message.Emote = "emoji"
		if len(run.Emoji.Shortcuts) > 0 {
			message.Emote = strings.Trim(run.Emoji.Shortcuts[0], ":")
		}
	}
	return message, true
}

func (s *Source) poll(ctx context.Context, token, version string) (pollAnswer, error) {
	body, _ := json.Marshal(map[string]any{
		"context":      map[string]any{"client": map[string]any{"clientName": "WEB", "clientVersion": version}},
		"continuation": token,
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.youtube.com/youtubei/v1/live_chat/get_live_chat?prettyPrint=false", bytes.NewReader(body))
	if err != nil {
		return pollAnswer{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", userAgent)
	response, err := s.http.Do(request)
	if err != nil {
		return pollAnswer{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return pollAnswer{}, fmt.Errorf("get_live_chat answered %d", response.StatusCode)
	}
	var answer pollAnswer
	err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&answer)
	return answer, err
}

func (s *Source) get(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept-Language", "en")
	request.Header.Set("Cookie", consentCookie)
	response, err := s.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %d", url, response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 16<<20))
}

func sleep(ctx context.Context, wait time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(wait):
		return true
	}
}
