import '@fontsource/share-tech-mono';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { createRoot } from 'react-dom/client';
import { useAppEvents } from '../useAppEvents';
import {
  CHAT_LABELS,
  CHAT_LINES_RANGE,
  CHAT_SIZE_RANGE,
  LABELS,
  MAX_PER_LIST,
  NUMBER_RANGES,
  parseChannel,
  parseKickChat,
  parseTwitchChat,
  parseYouTubeChat,
  type ChatOptions,
  type OverlayOptions,
  type Settings,
  type WatchersSnapshot,
} from '../watchers/options';
import './setup.css';

type ChoiceField = keyof typeof LABELS;
type NumberField = keyof typeof NUMBER_RANGES;

const CHOICES: { name: ChoiceField; label: string }[] = [
  { name: 'show', label: 'Draw' },
  { name: 'layout', label: 'Arrange' },
  { name: 'names', label: 'Names' },
  { name: 'enter', label: 'Arrive' },
  { name: 'motion', label: 'Move' },
  { name: 'color', label: 'Name colour' },
  { name: 'marks', label: 'Platform marks' },
];

const NUMBERS: { name: NumberField; label: string; hint: string }[] = [
  { name: 'window', label: 'Minutes', hint: 'How recently somebody must have spoken, for recent speakers.' },
  { name: 'max', label: 'At most', hint: 'Twelve suits a 1920 × 1080 source.' },
  { name: 'size', label: 'Size %', hint: 'Emote and name together.' },
  { name: 'speed', label: 'Speed %', hint: '200 is twice as quick, 25 a crawl.' },
  { name: 'roam', label: 'Roam %', hint: 'How far everybody wanders. 0 holds them still.' },
  { name: 'inset', label: 'Edge inset %', hint: 'Kept clear at every edge so nothing is cropped.' },
];

/** The three lists as the setup page writes them: one entry per line. */
interface Lists {
  embeds: string;
  kickChats: string;
  youtubeChats: string;
  twitchChats: string;
}

function listsText(settings: Settings): Lists {
  return {
    embeds: settings.embeds.map((embed) => `${embed.platform}/${embed.id}`).join('\n'),
    kickChats: settings.kickChats.join('\n'),
    youtubeChats: settings.youtubeChats.join('\n'),
    twitchChats: settings.twitchChats.join('\n'),
  };
}

function lines(text: string): string[] {
  return text.split('\n').map((line) => line.trim()).filter(Boolean);
}

/** A YouTube handle with whether something is live on it. */
function withLive(name: string, entry: string, snapshot: WatchersSnapshot): string {
  if (!(entry in snapshot.liveVideos)) return name;
  return snapshot.liveVideos[entry] ? `${name} (live)` : `${name} (not live)`;
}

function chatStatus(snapshot: WatchersSnapshot | null, reachable: boolean): string {
  if (!reachable) return 'The app is not running. Start it again and this page reconnects.';
  if (!snapshot) return 'Connecting…';
  const followed = [
    ...snapshot.embeds.map((embed) => withLive(`${embed.platform}/${embed.id} on destiny.gg`, embed.id, snapshot)),
    ...snapshot.kickChats.map((slug) => `Kick chat ${slug}`),
    ...snapshot.youtubeChats.map((entry) => withLive(`YouTube chat ${entry}`, entry, snapshot)),
    ...snapshot.twitchChats.map((login) => `Twitch chat ${login}`),
  ];
  if (followed.length === 0) return 'Choose an embed or a chat to start.';
  if (snapshot.embeds.length > 0 && !snapshot.connected) {
    return `Connecting to destiny.gg chat… Following ${followed.join(', ')}.`;
  }
  const people = snapshot.chatCount === 1 ? '1 person' : `${snapshot.chatCount} people`;
  return `${people} watching. Following ${followed.join(', ')}.`;
}

/** One list of entries, one per line. */
function EntryList({
  title,
  label,
  value,
  placeholder,
  hint,
  onChange,
}: {
  title: string;
  label: string;
  value: string;
  placeholder: string;
  hint: string;
  onChange: (value: string) => void;
}) {
  return (
    <section>
      <h2>{title}</h2>
      <label>
        {label}
        <textarea
          value={value}
          rows={Math.min(MAX_PER_LIST, Math.max(2, value.split('\n').length + 1))}
          placeholder={placeholder}
          spellCheck={false}
          onChange={(event) => onChange(event.currentTarget.value)}
        />
      </label>
      <p className="hint">{hint}</p>
    </section>
  );
}

interface UpdateState {
  current: string;
  /** A newer version already on disk, waiting for a restart. */
  installed: string;
  error: string;
}

/** The app's version and whether an update is waiting. Updates arrive rarely, so once a minute is plenty. */
function useUpdateState(): UpdateState | null {
  const [state, setState] = useState<UpdateState | null>(null);
  useEffect(() => {
    const load = () =>
      fetch('/api/status')
        .then((response) => (response.ok ? response.json() : null))
        .then((status: { update: UpdateState } | null) => status && setState(status.update))
        .catch(() => undefined);
    void load();
    const timer = window.setInterval(load, 60_000);
    return () => window.clearInterval(timer);
  }, []);
  return state;
}

function UpdateNotice({ update }: { update: UpdateState | null }) {
  const [restarting, setRestarting] = useState(false);
  if (!update?.installed) return null;

  async function restart() {
    setRestarting(true);
    await fetch('/api/restart', { method: 'POST', headers: { 'Content-Type': 'application/json' } }).catch(
      () => undefined,
    );
    // The new copy takes a moment to start listening, then this page reloads into it.
    window.setTimeout(() => window.location.reload(), 2_000);
  }

  return (
    <p className="notice">
      Version {update.installed} is installed. Restart to switch to it; OBS reconnects by itself.{' '}
      <button type="button" onClick={restart} disabled={restarting}>
        {restarting ? 'Restarting…' : 'Restart'}
      </button>
    </p>
  );
}

/** The overlay is laid out for 1920 × 1080; the preview draws it at that size and scales it to fit. */
function Preview() {
  const box = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(0.4);

  useEffect(() => {
    const element = box.current;
    if (!element) return;
    const observer = new ResizeObserver(() => setScale(element.clientWidth / 1_920));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return (
    <div className="preview" ref={box}>
      <iframe src="/watchers" title="Overlay preview" style={{ transform: `scale(${scale})` }} />
    </div>
  );
}

/** One OBS browser source address, with a button to copy it. */
function SourceUrl({ title, path, hint, open }: { title: string; path: string; hint: string; open?: boolean }) {
  const [copied, setCopied] = useState(false);
  const url = `${window.location.origin}${path}`;

  async function copy() {
    await navigator.clipboard.writeText(url);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1_500);
  }

  return (
    <div className="source">
      <h3>{title}</h3>
      <div className="row">
        <input className="url" readOnly value={url} onFocus={(event) => event.currentTarget.select()} />
        <button type="button" onClick={copy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
        {open && (
          <a className="button" href={path} target="_blank" rel="noreferrer">
            Open
          </a>
        )}
      </div>
      <p className="hint">{hint}</p>
    </div>
  );
}

const REPOSITORY = 'https://github.com/StrawWaffle/dgg-obs-tools';

type ChatChoice = keyof typeof CHAT_LABELS;

const CHAT_CHOICES: { name: ChatChoice; label: string }[] = [
  { name: 'dgg', label: 'destiny.gg messages' },
  { name: 'highlightWatching', label: 'Embed watchers' },
  { name: 'icons', label: 'Platform icons' },
  { name: 'badges', label: 'Badges' },
  { name: 'censor', label: 'Swear words' },
  { name: 'group', label: 'Runs of messages' },
  { name: 'shadow', label: 'Text shadow' },
  { name: 'links', label: 'Links' },
  { name: 'removed', label: 'Removed by a moderator' },
];

function ChatFields({ chat, onChange }: { chat: ChatOptions; onChange: (chat: ChatOptions) => void }) {
  return (
    <section>
      <h2>Chat</h2>
      <div className="fields">
        {CHAT_CHOICES.map(({ name, label }) => (
          <label key={name}>
            {label}
            <select value={chat[name]} onChange={(event) => onChange({ ...chat, [name]: event.currentTarget.value })}>
              {Object.entries(CHAT_LABELS[name]).map(([value, text]) => (
                <option key={value} value={value}>
                  {text}
                </option>
              ))}
            </select>
          </label>
        ))}
        <label title="Text and emotes together.">
          Size %
          <input
            type="number"
            min={CHAT_SIZE_RANGE.min}
            max={CHAT_SIZE_RANGE.max}
            value={chat.size}
            onChange={(event) => onChange({ ...chat, size: event.currentTarget.valueAsNumber || 0 })}
          />
        </label>
        <label title="How many messages the page keeps on screen.">
          Messages kept
          <input
            type="number"
            min={CHAT_LINES_RANGE.min}
            max={CHAT_LINES_RANGE.max}
            value={chat.lines}
            onChange={(event) => onChange({ ...chat, lines: event.currentTarget.valueAsNumber || 0 })}
          />
        </label>
      </div>
      <div className="checks">
        <label className="check">
          <input type="checkbox" checked={chat.kick} onChange={(event) => onChange({ ...chat, kick: event.currentTarget.checked })} />
          Show the Kick chats
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={chat.youtube}
            onChange={(event) => onChange({ ...chat, youtube: event.currentTarget.checked })}
          />
          Show the YouTube chats
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={chat.twitch}
            onChange={(event) => onChange({ ...chat, twitch: event.currentTarget.checked })}
          />
          Show the Twitch chats
        </label>
      </div>
      <label className="highlights">
        Highlighted phrases, one per line
        <textarea
          value={chat.highlights.join('\n')}
          rows={Math.min(8, Math.max(2, chat.highlights.length + 1))}
          spellCheck={false}
          onChange={(event) => onChange({ ...chat, highlights: event.currentTarget.value.split('\n') })}
        />
      </label>
      <p className="hint">A message mentioning one of these, in its text or its sender's name, stands out.</p>
    </section>
  );
}

function SetupPage() {
  const { settings, snapshot, reachable } = useAppEvents();
  const update = useUpdateState();
  const [lists, setLists] = useState<Lists>({ embeds: '', kickChats: '', youtubeChats: '', twitchChats: '' });
  const [overlay, setOverlay] = useState<OverlayOptions | null>(null);
  const [chat, setChat] = useState<ChatOptions | null>(null);
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);

  // Fill the form once, from what is saved. Later pushes are this page's own saves.
  useEffect(() => {
    if (settings && overlay === null) {
      setLists(listsText(settings));
      setOverlay(settings.overlay);
      setChat(settings.chat);
    }
  }, [settings, overlay]);

  async function save(event: FormEvent) {
    event.preventDefault();
    if (!overlay || !chat) return;
    const checks = [
      { entries: lines(lists.embeds), parse: parseChannel, what: 'an embed', how: 'platform/name, such as kick/destiny' },
      { entries: lines(lists.kickChats), parse: parseKickChat, what: 'a Kick channel', how: 'a channel name, such as destiny' },
      {
        entries: lines(lists.youtubeChats),
        parse: parseYouTubeChat,
        what: 'a YouTube chat',
        how: 'a handle such as @destiny, or a video id',
      },
      { entries: lines(lists.twitchChats), parse: parseTwitchChat, what: 'a Twitch channel', how: 'a channel name, such as xqc' },
    ];
    for (const { entries, parse, what, how } of checks) {
      const unreadable = entries.find((entry) => !parse(entry));
      if (unreadable) {
        setMessage({ text: `“${unreadable}” isn't ${what}. Write each as ${how}, or paste a link.`, error: true });
        return;
      }
      if (entries.length > MAX_PER_LIST) {
        setMessage({ text: `Follow at most ${MAX_PER_LIST} of each.`, error: true });
        return;
      }
    }
    const next: Settings = {
      embeds: lines(lists.embeds).map((entry) => parseChannel(entry)!),
      kickChats: lines(lists.kickChats).map((entry) => parseKickChat(entry)!),
      youtubeChats: lines(lists.youtubeChats).map((entry) => parseYouTubeChat(entry)!),
      twitchChats: lines(lists.twitchChats).map((entry) => parseTwitchChat(entry)!),
      overlay,
      chat: { ...chat, highlights: lines(chat.highlights.join('\n')) },
    };
    const response = await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(next),
    }).catch(() => null);
    if (!response?.ok) {
      setMessage({ text: response ? await response.text() : 'The app is not answering.', error: true });
      return;
    }
    const saved = (await response.json()) as Settings;
    setLists(listsText(saved));
    setChat(saved.chat);
    setMessage({ text: 'Saved. Open overlays and chats update straight away.', error: false });
  }

  async function quit() {
    await fetch('/api/quit', { method: 'POST', headers: { 'Content-Type': 'application/json' } }).catch(
      () => undefined,
    );
  }

  if (!overlay || !chat) {
    return (
      <main className="setup">
        <p className="status">{chatStatus(snapshot, reachable)}</p>
      </main>
    );
  }

  return (
    <main className="setup">
      <header className="page-head">
        <h1>
          dgg-obs-tools <span className="version">{update?.current}</span>
        </h1>
        <nav>
          <a href={REPOSITORY} target="_blank" rel="noreferrer">
            GitHub
          </a>
          <a href={`${REPOSITORY}/issues/new/choose`} target="_blank" rel="noreferrer">
            Report a problem or ask for a feature
          </a>
        </nav>
      </header>
      <p className="status">{chatStatus(snapshot, reachable)}</p>
      <UpdateNotice update={update} />

      <section>
        <h2>OBS browser sources</h2>
        <SourceUrl
          title="Watchers overlay"
          path="/watchers"
          hint="Add a Browser source in OBS with this URL, sized 1920 × 1080. It draws on a transparent background over whatever is below it."
        />
        <SourceUrl
          title="Chat"
          path="/chat"
          open
          hint="Everything said in the followed chats, in one list. In OBS it draws on a transparent background; size the source to the column you want. Open it in a browser to read it yourself. Keep this app running while you use either."
        />
      </section>

      <form onSubmit={save}>
        <EntryList
          title="destiny.gg embeds"
          label="Everyone in destiny.gg chat watching these embeds, one per line"
          value={lists.embeds}
          placeholder="kick/destiny"
          hint="Each as platform/name: kick/destiny, youtube/@destiny (follows whatever that channel has live) or youtube/VIDEO_ID. A destiny.gg bigscreen link works too."
          onChange={(embeds) => setLists({ ...lists, embeds })}
        />
        <EntryList
          title="Kick chats"
          label="Everyone talking in these Kick channels' chat, one per line"
          value={lists.kickChats}
          placeholder="destiny"
          hint="A channel name, or a kick.com link."
          onChange={(kickChats) => setLists({ ...lists, kickChats })}
        />
        <EntryList
          title="YouTube chats"
          label="Everyone talking in these YouTube chats, one per line"
          value={lists.youtubeChats}
          placeholder="@destiny"
          hint="A channel handle such as @destiny, which follows whatever that channel has live, or a video id or link."
          onChange={(youtubeChats) => setLists({ ...lists, youtubeChats })}
        />
        <EntryList
          title="Twitch chats"
          label="Everyone talking in these Twitch channels' chat, one per line"
          value={lists.twitchChats}
          placeholder="xqc"
          hint="A channel name, or a twitch.tv link. Everything above feeds both the overlay and the chat."
          onChange={(twitchChats) => setLists({ ...lists, twitchChats })}
        />

        <section>
          <h2>Watchers overlay</h2>
          <div className="fields">
            {CHOICES.map(({ name, label }) => (
              <label key={name}>
                {label}
                <select
                  value={overlay[name]}
                  onChange={(event) => setOverlay({ ...overlay, [name]: event.currentTarget.value })}
                >
                  {Object.entries(LABELS[name]).map(([value, text]) => (
                    <option key={value} value={value}>
                      {text}
                    </option>
                  ))}
                </select>
              </label>
            ))}
            {NUMBERS.map(({ name, label, hint }) => (
              <label key={name} title={hint}>
                {label}
                <input
                  type="number"
                  min={NUMBER_RANGES[name].min}
                  max={NUMBER_RANGES[name].max}
                  value={overlay[name]}
                  onChange={(event) =>
                    setOverlay({ ...overlay, [name]: event.currentTarget.valueAsNumber || 0 })
                  }
                />
              </label>
            ))}
          </div>
        </section>

        <ChatFields chat={chat} onChange={setChat} />

        <div className="row">
          <button type="submit" className="primary">
            Save
          </button>
          {message && <p className={message.error ? 'message error' : 'message'}>{message.text}</p>}
        </div>
      </form>

      <section className="preview-section">
        <h2>Overlay preview</h2>
        <Preview />
      </section>

      <section>
        <h2>Running</h2>
        <div className="row">
          <button type="button" onClick={quit}>
            Quit
          </button>
          <p className="hint inline">
            The app sits in the system tray (the menu bar on a Mac). Quitting stops the overlay and the chat.
          </p>
        </div>
      </section>
    </main>
  );
}

createRoot(document.getElementById('root')!).render(<SetupPage />);
