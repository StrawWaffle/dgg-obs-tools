// dgg-obs-tools serves OBS browser sources that draw destiny.gg chat on
// stream. Run it, and it opens its setup page and sits in the system tray;
// paste the overlay URL from the setup page into an OBS browser source.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"time"

	"github.com/StrawWaffle/dgg-obs-tools/internal/server"
	"github.com/StrawWaffle/dgg-obs-tools/internal/settings"
	"github.com/StrawWaffle/dgg-obs-tools/internal/tray"
	"github.com/StrawWaffle/dgg-obs-tools/internal/update"
	"github.com/StrawWaffle/dgg-obs-tools/internal/watchers"
	"github.com/StrawWaffle/dgg-obs-tools/web"
)

// version is set by the release build.
var version = "dev"

func main() {
	port := flag.Int("port", 7341, "port to serve on, on this machine only")
	noBrowser := flag.Bool("no-browser", false, "do not open the setup page on start")
	flag.Parse()

	address := fmt.Sprintf("127.0.0.1:%d", *port)
	setupURL := fmt.Sprintf("http://localhost:%d/", *port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		// Most likely this is already running, started from a shortcut or at
		// login. Show its setup page rather than failing.
		if alreadyRunning(address) {
			fmt.Println("dgg-obs-tools is already running at", setupURL)
			if !*noBrowser {
				openBrowser(setupURL)
			}
			return
		}
		log.Fatalf("could not listen on %s: %v (try -port with another number)", address, err)
	}

	settingsPath, err := settings.Path()
	if err != nil {
		log.Fatalf("no settings folder: %v", err)
	}

	logPath := logToFile(settingsPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tracker := watchers.New(settingsPath)
	go tracker.Run(ctx)

	// Taken now, because an update renames (Windows) or replaces (elsewhere)
	// the running file, after which the system may report the old one.
	executable, err := os.Executable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		log.Fatalf("could not find this program's own file: %v", err)
	}

	update.RemoveOld(executable)
	updater := update.New(version, executable)
	go updater.Run(ctx)

	// Everything that stops the app goes through the tray, which owns the main
	// goroutine: Quit in its menu, the setup page's Quit button, Ctrl+C in a
	// terminal, or a restart into an update.
	var restarting atomic.Bool
	restart := func() {
		restarting.Store(true)
		tray.Stop()
	}
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	go func() {
		<-interrupts
		tray.Stop()
	}()

	httpServer := &http.Server{
		Handler:           server.New(tracker, updater, restart, tray.Stop, web.Pages()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	fmt.Println("dgg-obs-tools", version)
	fmt.Println("Setup page:        ", setupURL)
	fmt.Println("OBS browser source:", setupURL+"watchers")
	fmt.Println("Settings saved in: ", settingsPath)
	fmt.Println("Log:               ", logPath)
	fmt.Println("It runs in the system tray; quit it from there.")
	if !*noBrowser {
		openBrowser(setupURL)
	}

	tray.Run(tray.Options{
		OpenSetup: func() { openBrowser(setupURL) },
		Version:   version,
		Update: func() (string, error) {
			return updater.CheckNow(ctx)
		},
		Installed: func() string { return updater.State().Installed },
		Restart:   restart,
		Quit:      tray.Stop,
	})

	cancel()
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShutdown()
	_ = httpServer.Shutdown(shutdown)
	if restarting.Load() {
		relaunch(executable)
	}
}

// logToFile copies the log into a file beside the settings. A release build on
// Windows has no console, so this is the only place a problem can be read.
// The file is started fresh each launch, which keeps it small.
func logToFile(settingsPath string) string {
	path := filepath.Join(filepath.Dir(settingsPath), "dgg-obs-tools.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ""
	}
	file, err := os.Create(path)
	if err != nil {
		return ""
	}
	// The file first: without a console, writing to stderr fails, and a
	// MultiWriter stops at the first failure.
	log.SetOutput(io.MultiWriter(file, os.Stderr))
	return path
}

// relaunch starts the file on disk again, which after an update is the new
// version, once this copy has let go of the port. The pages that were open
// reconnect to it by themselves, so it does not open another one.
func relaunch(executable string) {
	args := os.Args[1:]
	if !slices.Contains(args, "-no-browser") {
		args = append(args, "-no-browser")
	}
	command := exec.Command(executable, args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		log.Fatalf("could not restart: %v", err)
	}
	log.Print("restarted as the updated version")
}

func alreadyRunning(address string) bool {
	client := http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + address + "/api/status")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	var status server.Status
	return json.NewDecoder(response.Body).Decode(&status) == nil && status.App == server.AppName
}

func openBrowser(url string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		command = exec.Command("open", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	if err := command.Start(); err != nil {
		log.Printf("could not open a browser: %v", err)
	}
}
