// Package tray puts the app in the system tray (the menu bar on macOS), so it
// runs without a window of its own.
package tray

import (
	"runtime"
	"time"

	"fyne.io/systray"

	"github.com/StrawWaffle/dgg-obs-tools/assets"
)

// Options is what the menu can do.
type Options struct {
	OpenSetup func()
	// Version is the running version, shown when there is nothing newer.
	Version string
	// Update looks for a newer release now and installs it, answering the
	// version waiting for a restart, or "" when there is none.
	Update func() (string, error)
	// Installed answers a version the background check has already installed.
	Installed func() string
	Restart   func()
	Quit      func()
}

const checkTitle = "Check for updates"

// Run shows the icon and blocks until Stop is called. It must be called from
// the main goroutine, because macOS only draws menus from the main thread.
func Run(options Options) {
	systray.Run(func() { ready(options) }, func() {})
}

// Stop removes the icon and makes Run return.
func Stop() {
	systray.Quit()
}

func ready(options Options) {
	switch runtime.GOOS {
	case "windows":
		systray.SetIcon(assets.TrayICO)
	case "darwin":
		systray.SetTemplateIcon(assets.TrayTemplatePNG, assets.TrayPNG)
	default:
		systray.SetIcon(assets.TrayPNG)
	}
	systray.SetTooltip("dgg-obs-tools " + options.Version)
	// A left click opens the setup page, which is what almost everyone wants;
	// the menu is on right click.
	systray.SetOnTapped(options.OpenSetup)

	open := systray.AddMenuItem("Open setup page", "Change what is followed and how the overlay looks")
	update := systray.AddMenuItem(checkTitle, "Download the newest version and restart into it")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Stop serving the OBS overlays")

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				options.OpenSetup()
			case <-update.ClickedCh:
				go updateNow(update, options)
			case <-quit.ClickedCh:
				options.Quit()
			}
		}
	}()

	// The background check installs updates by itself; say so once it has.
	go func() {
		for {
			if installed := options.Installed(); installed != "" {
				update.SetTitle("Restart to update to " + installed)
				return
			}
			time.Sleep(time.Minute)
		}
	}()
}

// updateNow is the menu item: it restarts into an update already on disk, or
// looks for one, installs it and restarts. With nothing newer it says so for a
// few seconds. Asking for it is asking for the restart, so it does not wait
// for a second click.
func updateNow(item *systray.MenuItem, options Options) {
	item.Disable()
	defer item.Enable()
	item.SetTitle("Checking for updates…")
	installed, err := options.Update()
	switch {
	case installed != "":
		item.SetTitle("Restarting into " + installed + "…")
		options.Restart()
		return
	case err != nil:
		item.SetTitle("Update check failed; see the log")
	default:
		item.SetTitle("Up to date (" + options.Version + ")")
	}
	time.Sleep(5 * time.Second)
	item.SetTitle(checkTitle)
}
