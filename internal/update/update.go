// Package update replaces the running executable with the newest release from
// GitHub, so a portable copy stays current without an installer.
package update

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/creativeprojects/go-selfupdate"
)

const (
	repository = "StrawWaffle/dgg-obs-tools"
	// GitHub allows 60 anonymous API calls an hour per address. A few a day is
	// plenty for something that changes this rarely.
	checkEvery = 6 * time.Hour
	retryAfter = 30 * time.Minute
)

// State is what the setup page shows about updates.
type State struct {
	Current string `json:"current"`
	// The newer version now on disk, waiting for a restart. Empty while there is none.
	Installed string `json:"installed"`
	Error     string `json:"error"`
}

// Updater checks for releases in the background and installs a newer one in
// place of the running file. The running copy carries on until it is
// restarted, so an update never cuts a stream short by itself.
type Updater struct {
	mu    sync.Mutex
	state State
}

// New starts with the running version. "dev" builds never update.
func New(current string) *Updater {
	return &Updater{state: State{Current: current}}
}

// State returns the current state.
func (u *Updater) State() State {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

// Run checks now and then every few hours until an update is installed over
// executable or ctx ends.
func (u *Updater) Run(ctx context.Context, executable string) {
	if u.state.Current == "dev" {
		return
	}
	for {
		wait := checkEvery
		installed, err := u.check(ctx, executable)
		u.mu.Lock()
		if err != nil {
			u.state.Error = err.Error()
			wait = retryAfter
			log.Printf("update check: %v", err)
		} else {
			u.state.Error = ""
		}
		if installed != "" {
			u.state.Installed = installed
		}
		u.mu.Unlock()
		if installed != "" || ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (u *Updater) check(ctx context.Context, executable string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		// Every release carries checksums.txt, so a damaged or swapped download
		// is refused rather than run.
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return "", err
	}
	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repository))
	if err != nil || !found || latest.LessOrEqual(u.state.Current) {
		return "", err
	}
	if err := updater.UpdateTo(ctx, latest, executable); err != nil {
		return "", err
	}
	log.Printf("installed %s; restart to use it", latest.Version())
	return latest.Version(), nil
}
