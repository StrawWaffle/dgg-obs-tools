// Package server serves the setup page and the overlays to the local machine.
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/StrawWaffle/dgg-obs-tools/internal/settings"
	"github.com/StrawWaffle/dgg-obs-tools/internal/update"
	"github.com/StrawWaffle/dgg-obs-tools/internal/watchers"
)

// Status identifies a running copy, so a second launch can find the first.
type Status struct {
	App    string       `json:"app"`
	Update update.State `json:"update"`
}

// AppName is what Status reports in App.
const AppName = "dgg-obs-tools"

// New builds the handler. pages is the built web app: index.html (setup),
// watchers.html (the overlay) and their assets. restart is called once the
// response to a restart request has gone out.
func New(tracker *watchers.Tracker, updater *update.Updater, restart func(), pages fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, Status{App: AppName, Update: updater.State()})
	})

	// Switches to an update that is already on disk. Open pages reconnect by
	// themselves once the new copy is listening.
	mux.HandleFunc("POST /api/restart", func(w http.ResponseWriter, r *http.Request) {
		if !isJSON(r) {
			http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		go restart()
	})

	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, tracker.Settings())
	})

	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		if !isJSON(r) {
			http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
			return
		}
		var next settings.Settings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&next); err != nil {
			http.Error(w, "settings could not be read", http.StatusBadRequest)
			return
		}
		if err := tracker.SetSettings(next); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, tracker.Settings())
	})

	// Server-sent events rather than a WebSocket: one direction is all an
	// overlay needs, and the browser reconnects an EventSource by itself.
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")

		events, unsubscribe := tracker.Subscribe()
		defer unsubscribe()
		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-events:
				data, err := json.Marshal(event.Data)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})

	static := http.FileServerFS(pages)
	mux.HandleFunc("GET /watchers", func(w http.ResponseWriter, r *http.Request) {
		// Every option lives in the saved settings, so the overlay page never
		// changes and an OBS cache can keep it.
		http.ServeFileFS(w, r, pages, "watchers.html")
	})
	mux.Handle("GET /", static)

	return localOnly(mux)
}

// localOnly refuses requests whose Host is not this machine. The server only
// listens on loopback, but a hostile site can point its own domain at
// 127.0.0.1 and then read the answers as same-origin; checking Host stops that.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if host != "localhost" && net.ParseIP(host) == nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isJSON guards the routes that change something. A page on another site can
// send a form to localhost, but not this content type without a preflight,
// which nothing here answers.
func isJSON(r *http.Request) bool {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return mediaType == "application/json"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
