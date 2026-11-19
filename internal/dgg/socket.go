// Package dgg reads destiny.gg's public chat socket anonymously.
package dgg

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// UserAgent names this program to destiny.gg, so whoever runs the servers can
// see what is connecting and where to find it.
const UserAgent = "dgg-obs-tools (+https://github.com/StrawWaffle/dgg-obs-tools)"

const (
	// The backend completes the handshake before it authenticates, so a
	// connection it then rejects opens first. Only one that stays up this long
	// resets the backoff, or a rejected client would retry at the shortest delay
	// forever.
	stableAfter = 5 * time.Second

	retryWindow    = 3 * time.Second
	retryWindowCap = 60 * time.Second
	retryMin       = 500 * time.Millisecond

	// Cloudflare cycling a server closes every client on it at once with 1001,
	// so even a fast retry needs spreading out.
	goingAwaySpread = time.Second

	// The chat server sends a protocol ping every 10 seconds. Half a minute
	// without any frame means the connection is gone, whatever it claims.
	silenceLimit = 30 * time.Second
	silenceCheck = 5 * time.Second

	handshakeTimeout = 10 * time.Second

	// NAMES carries the whole roster, a few hundred kilobytes at busy times.
	readLimit = 8 << 20
)

// retryDelay is chat-gui's backoff: full jitter over a window that doubles with
// each failed attempt. The jitter matters as much as the backoff, because a
// mass disconnect releases every client at the same moment.
func retryDelay(attempts int, random func() float64) time.Duration {
	window := retryWindowCap
	if attempts < 6 {
		window = min(retryWindowCap, retryWindow*time.Duration(1<<attempts))
	}
	return retryMin + time.Duration(random()*float64(window-retryMin))
}

// SocketState is what the setup page shows about the connection.
type SocketState struct {
	Connected bool `json:"connected"`
	Attempts  int  `json:"attempts"`
}

// Socket is one reconnecting connection to a destiny.gg socket.
//
// It sends no Origin header. destiny.gg answers 403 to any origin but its own
// and accepts a handshake with none, which is why this has to run outside a
// browser: a page always sends its origin and cannot be told not to.
type Socket struct {
	URL string
	// OnFrame receives every text frame as it arrived.
	OnFrame func(raw string)

	mu        sync.Mutex
	conn      *websocket.Conn
	attempts  int
	connected bool
	restart   chan struct{}
}

// Run connects and keeps reconnecting until ctx ends.
func (s *Socket) Run(ctx context.Context) {
	s.mu.Lock()
	s.restart = make(chan struct{}, 1)
	s.mu.Unlock()

	for ctx.Err() == nil {
		s.mu.Lock()
		s.attempts++
		attempts := s.attempts
		s.mu.Unlock()

		openedAt, code, restarted := s.connectOnce(ctx)

		stable := !openedAt.IsZero() && time.Since(openedAt) >= stableAfter
		if stable || restarted {
			s.mu.Lock()
			s.attempts = 0
			s.mu.Unlock()
		}

		var delay time.Duration
		switch {
		case restarted:
			delay = 0
		case code == websocket.StatusGoingAway && stable:
			delay = time.Duration(rand.Float64() * float64(goingAwaySpread))
		case stable:
			delay = retryDelay(0, rand.Float64)
		default:
			delay = retryDelay(attempts, rand.Float64)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// Reconnect drops the current connection and opens a new one straight away.
// destiny.gg only sends the roster on connect, so this is how a new channel
// gets a full list instead of filling up one message at a time.
func (s *Socket) Reconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.restart == nil {
		return
	}
	select {
	case s.restart <- struct{}{}:
	default:
	}
}

// Send writes one text frame if the connection is open.
func (s *Socket) Send(ctx context.Context, raw string) {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, websocket.MessageText, []byte(raw))
}

// State reports whether the socket is up and how many attempts it has made.
func (s *Socket) State() SocketState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SocketState{Connected: s.connected, Attempts: s.attempts}
}

// connectOnce holds one connection until it fails, goes quiet, is asked to
// restart, or ctx ends. It reports when it opened (zero if it never did), the
// close code, and whether it ended because Reconnect asked it to.
func (s *Socket) connectOnce(ctx context.Context) (time.Time, websocket.StatusCode, bool) {
	var lastFrame atomic.Int64
	touch := func() { lastFrame.Store(time.Now().UnixNano()) }

	dialCtx, cancelDial := context.WithTimeout(ctx, handshakeTimeout)
	conn, _, err := websocket.Dial(dialCtx, s.URL, &websocket.DialOptions{
		HTTPHeader: http.Header{"User-Agent": []string{UserAgent}},
		OnPingReceived: func(context.Context, []byte) bool {
			touch()
			return true
		},
		OnPongReceived: func(context.Context, []byte) { touch() },
	})
	cancelDial()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("%s: %v", s.URL, err)
		}
		return time.Time{}, -1, false
	}
	conn.SetReadLimit(readLimit)

	openedAt := time.Now()
	touch()
	s.mu.Lock()
	s.conn = conn
	s.connected = true
	restart := s.restart
	s.mu.Unlock()
	// A restart asked for while there was no connection is already answered:
	// this one is new and will get the roster.
	select {
	case <-restart:
	default:
	}

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var restarted, silent atomic.Bool
	go func() {
		ticker := time.NewTicker(silenceCheck)
		defer ticker.Stop()
		for {
			select {
			case <-connCtx.Done():
				return
			case <-restart:
				restarted.Store(true)
				_ = conn.CloseNow()
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, lastFrame.Load())) >= silenceLimit {
					silent.Store(true)
					log.Printf("%s went quiet, reconnecting", s.URL)
					_ = conn.CloseNow()
					return
				}
			}
		}
	}()

	var readErr error
	for {
		kind, data, err := conn.Read(connCtx)
		if err != nil {
			readErr = err
			break
		}
		touch()
		if kind == websocket.MessageText {
			s.OnFrame(string(data))
		}
	}

	s.mu.Lock()
	s.conn = nil
	s.connected = false
	s.mu.Unlock()
	_ = conn.CloseNow()

	code := websocket.CloseStatus(readErr)
	if ctx.Err() == nil && !restarted.Load() && !silent.Load() && code != websocket.StatusGoingAway {
		var closeErr websocket.CloseError
		if !errors.As(readErr, &closeErr) {
			log.Printf("%s: %v", s.URL, readErr)
		}
	}
	return openedAt, code, restarted.Load()
}
