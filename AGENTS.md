# Project instructions

## Identity

- Published as **StrawWaffle**. Commit only with this repo's local
  `user.name`/`user.email`, and add no co-author or attribution lines; the
  hooks in `.git/hooks` reject them.
- Use only StrawWaffle's credentials for GitHub. Don't use the `gh` CLI here
  unless it is logged in as StrawWaffle.
- No links to, or mentions of, other projects or accounts of the owner.

## Engineering

- Keep it small: one portable executable per platform, no installer, no
  runtime to install. Prefer the standard library and the dependencies already
  here before adding one.
- Grow in layers. Each change should leave a working program.
- Remove obsolete paths instead of keeping compatibility layers.
- Be gentle with destiny.gg: one anonymous chat connection, held only while a
  page is open, with chat-gui's backoff on reconnect.

## Layout

- `main.go`: flags, single-instance check, HTTP server on loopback.
- `internal/dgg`: destiny.gg chat socket, roster, emote list, flair colours.
- `internal/kick`, `internal/youtube`, `internal/twitch`: anonymous Kick
  (Centrifugo), YouTube (live chat polling) and Twitch (IRC as justinfan) readers.
- `internal/chat`: the watcher type and backoff they share.
- `internal/watchers`: the tracker that holds the connections and feeds open
  pages: snapshots of who is watching to overlays (`tracker.go`), messages and
  removals to chat pages (`chatfeed.go`).
- `internal/settings`: saved settings and their limits (mirrored in
  `web/src/watchers/options.ts`).
- `internal/server`: routes, server-sent events, localhost guards.
- `web/`: the setup page, the watchers overlay and the chat (React, Vite),
  embedded into the executable from `web/dist`. `web/public/platform/` and
  `web/src/platform.tsx` hold the platform icons.

## Screenshots

`docs/screenshots/` holds the README's pictures, taken from real public chat.
Live chat always has something unfit for a README in it, so when retaking them
leave such lines out (the screenshots so far froze the page and removed those
lines before capturing) and check every name and message that ends up in the
picture.
