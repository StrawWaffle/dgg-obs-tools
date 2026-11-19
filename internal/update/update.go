// Package update replaces the running executable with the newest release from
// GitHub, so a portable copy stays current without an installer.
package update

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	executable string

	mu    sync.Mutex
	state State
	// Held while a check runs, so the background check and one asked for from
	// the tray never install over each other.
	checking sync.Mutex
}

// New starts with the running version and the file to replace. "dev" builds
// never update.
func New(current, executable string) *Updater {
	return &Updater{executable: executable, state: State{Current: current}}
}

// AssetOS is how a release file names an operating system: the word people
// know rather than Go's own, so `macos` instead of `darwin`.
// scripts/build-release.sh names the files the same way.
func AssetOS(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

// AssetArch is how a release file names a processor: `x64`, which is what
// Windows itself calls it, instead of Go's `amd64`, which reads as AMD-only.
func AssetArch(goarch string) string {
	if goarch == "amd64" {
		return "x64"
	}
	return goarch
}

// RemoveOld deletes the copy an earlier update left beside executable. Windows
// cannot delete a running file, so the update renames it to a hidden
// `.<name>.old` instead, and only a later start can remove it. After a
// restart the previous copy is still exiting when this one starts, so it keeps
// trying for a little while; if that is not enough, the next start tries again.
func RemoveOld(executable string) {
	dir, name := filepath.Split(executable)
	old := filepath.Join(dir, "."+name+".old")
	go func() {
		for range 15 {
			if err := os.Remove(old); err == nil || errors.Is(err, fs.ErrNotExist) {
				return
			}
			time.Sleep(time.Second)
		}
	}()
}

// State returns the current state.
func (u *Updater) State() State {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

// Run checks now and then every few hours until an update is installed or
// ctx ends.
func (u *Updater) Run(ctx context.Context) {
	for {
		installed, err := u.CheckNow(ctx)
		if installed != "" || ctx.Err() != nil || u.state.Current == "dev" {
			return
		}
		wait := checkEvery
		if err != nil {
			wait = retryAfter
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// CheckNow looks for a newer release straight away and installs it. It
// answers the version now on disk waiting for a restart, which may be one an
// earlier check installed, or "" when there is nothing newer.
func (u *Updater) CheckNow(ctx context.Context) (string, error) {
	if u.state.Current == "dev" {
		return "", nil
	}
	u.checking.Lock()
	defer u.checking.Unlock()
	if installed := u.State().Installed; installed != "" {
		return installed, nil
	}

	installed, err := u.install(ctx)
	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil {
		u.state.Error = err.Error()
		log.Printf("update check: %v", err)
	} else {
		u.state.Error = ""
	}
	if installed != "" {
		u.state.Installed = installed
	}
	return installed, err
}

func (u *Updater) install(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		OS:   AssetOS(runtime.GOOS),
		Arch: AssetArch(runtime.GOARCH),
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
	if err := updater.UpdateTo(ctx, latest, u.executable); err != nil {
		return "", err
	}
	log.Printf("installed %s; restart to use it", latest.Version())
	return latest.Version(), nil
}
