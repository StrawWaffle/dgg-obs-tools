# dgg-obs-tools

OBS browser sources that draw destiny.gg chat on your stream. It's a single
small program: run it, pick an embed, and paste one URL into OBS.

## Watchers

Shows the people in destiny.gg chat who have a given embed open (say
`kick/destiny`, or several embeds at once) as the last emote each of them used, with their name in its
chat flair colour. Layouts include free float, a safe frame around the edges,
a bottom rail, side columns and bumper cars, plus options for speed, size,
entrances and names.

Everything it shows is already public in destiny.gg chat. It connects
anonymously and needs no login.

## Download

Get the file for your system from the
[latest release](https://github.com/StrawWaffle/dgg-obs-tools/releases/latest).
There's nothing to install: put it anywhere and run it.

- **Windows:** `dgg-obs-tools_windows_x64.exe` (or `_arm64.exe` on a Snapdragon
  laptop). The file isn't signed, so the first time Windows may say it
  protected your PC: click **More info**, then **Run anyway**.
- **macOS:** `dgg-obs-tools_macos_arm64.tar.gz` (Apple silicon) or
  `_macos_x64` (Intel). Unpack it, then the first time right-click the file
  and choose **Open**, because it isn't signed.
- **Linux:** `dgg-obs-tools_linux_x64.tar.gz` (or `_arm64`). Unpack and run.

It keeps itself up to date. A few times a day it checks for a new release,
verifies it against the release's checksums and puts it in place of its own
file. It never restarts by itself, so an update can't interrupt a stream: the
setup page offers a **Restart** button, and otherwise the next launch uses the
new version.

## Setup

1. Run `dgg-obs-tools`. It opens its setup page in your browser at
   `http://localhost:7341/`.
2. Under **Embeds**, enter each embed on its own line as `platform/name`
   (`kick/destiny`, `youtube/VIDEO_ID`) or paste a `destiny.gg/bigscreen#...`
   link, then click **Save**. Several embeds are drawn together in one overlay.
3. In OBS add a **Browser** source with the URL `http://localhost:7341/watchers`,
   sized 1920 × 1080.

It runs in the system tray (the menu bar on a Mac), with no window of its
own: click the icon to open the setup page, and right-click it to quit. Keep it
running while you stream. Changes saved on the setup page show up in OBS
straight away. Running it a second time just opens the setup page of the copy
that's already running.

On Linux the icon needs a desktop that shows tray icons; on GNOME that means
the AppIndicator extension. Without one, the setup page has a **Quit** button.

Settings are kept in your user config folder (`%APPDATA%\dgg-obs-tools` on
Windows, `~/Library/Application Support/dgg-obs-tools` on macOS,
`~/.config/dgg-obs-tools` on Linux), so the program file can be moved or
replaced without losing them. The log of the latest run, `dgg-obs-tools.log`,
is kept in the same folder.

Options: `-port` serves on another port, and `-no-browser` skips opening the
setup page.

## Why it runs on your machine

destiny.gg's chat socket refuses connections from any web page that isn't
destiny.gg, and an OBS browser source is a web page. So something outside the
browser has to hold the connection. This program does that and serves the
overlay to OBS on `localhost` only.

## Building

Needs Go and Node.

```sh
cd web && npm ci && npm run build && cd ..
go build -trimpath -ldflags "-s -w" -o dgg-obs-tools .
```

`npm run dev` in `web/` serves the pages with hot reload and forwards `/api`
to a copy of the program running on the default port.

Tests: `go test ./...` and `npm test` in `web/`.

To publish a release, push a tag such as `v0.2.0`. The release workflow builds
every platform with `scripts/build-release.sh` and attaches the files and
`checksums.txt`. Copies built from source report version `dev` and never
update themselves.
