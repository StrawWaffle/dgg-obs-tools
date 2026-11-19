# dgg-obs-tools

OBS browser sources that draw destiny.gg chat on your stream. It's a single
small program: run it, pick an embed, and paste one URL into OBS.

## Watchers

Shows the people in destiny.gg chat who have a given embed open (say
`kick/destiny`) as the last emote each of them used, with their name in its
chat flair colour. Layouts include free float, a safe frame around the edges,
a bottom rail, side columns and bumper cars, plus options for speed, size,
entrances and names.

Everything it shows is already public in destiny.gg chat. It connects
anonymously and needs no login.

## Setup

1. Run `dgg-obs-tools`. It opens its setup page in your browser at
   `http://localhost:7341/`.
2. Under **Embed**, enter the embed as `platform/name` (`kick/destiny`,
   `youtube/VIDEO_ID`) or paste a `destiny.gg/bigscreen#...` link, then click **Save**.
3. In OBS add a **Browser** source with the URL `http://localhost:7341/watchers`,
   sized 1920 × 1080.

Keep the program running while you stream. Changes saved on the setup page
show up in OBS straight away. Running it a second time just opens the setup
page of the copy that's already running.

Settings are kept in your user config folder (`%APPDATA%\dgg-obs-tools` on
Windows, `~/Library/Application Support/dgg-obs-tools` on macOS,
`~/.config/dgg-obs-tools` on Linux), so the program file can be moved or
replaced without losing them.

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
