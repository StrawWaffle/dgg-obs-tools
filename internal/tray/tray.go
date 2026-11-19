// Package tray puts the app in the system tray (the menu bar on macOS), so it
// runs without a window of its own.
package tray

import (
	"runtime"
	"time"

	"fyne.io/systray"

	"github.com/StrawWaffle/dgg-obs-tools/assets"
	"github.com/StrawWaffle/dgg-obs-tools/internal/update"
)

// Options is what the menu can do.
type Options struct {
	OpenSetup func()
	Update    func() update.State
	Restart   func()
	Quit      func()
}

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
	systray.SetTooltip("dgg-obs-tools")
	// A left click opens the setup page, which is what almost everyone wants;
	// the menu is on right click.
	systray.SetOnTapped(options.OpenSetup)

	open := systray.AddMenuItem("Open setup page", "Change embeds and how the overlay looks")
	restart := systray.AddMenuItem("Restart to update", "Switch to the version already downloaded")
	restart.Hide()
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Stop serving the OBS overlays")

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				options.OpenSetup()
			case <-restart.ClickedCh:
				options.Restart()
			case <-quit.ClickedCh:
				options.Quit()
			}
		}
	}()

	// An update is installed in the background; offer it once it is.
	go func() {
		for {
			if installed := options.Update().Installed; installed != "" {
				restart.SetTitle("Restart to update to " + installed)
				restart.Show()
				return
			}
			time.Sleep(time.Minute)
		}
	}()
}
