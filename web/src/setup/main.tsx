import '@fontsource/share-tech-mono';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { createRoot } from 'react-dom/client';
import { useAppEvents } from '../useAppEvents';
import {
  LABELS,
  MAX_PER_LIST,
  NUMBER_RANGES,
  parseChannel,
  parseKickChat,
  parseYouTubeChat,
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
}

function listsText(settings: Settings): Lists {
  return {
    embeds: settings.embeds.map((embed) => `${embed.platform}/${embed.id}`).join('\n'),
    kickChats: settings.kickChats.join('\n'),
    youtubeChats: settings.youtubeChats.join('\n'),
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

function SetupPage() {
  const { settings, snapshot, reachable } = useAppEvents();
  const update = useUpdateState();
  const [lists, setLists] = useState<Lists>({ embeds: '', kickChats: '', youtubeChats: '' });
  const [overlay, setOverlay] = useState<OverlayOptions | null>(null);
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);
  const [copied, setCopied] = useState(false);

  // Fill the form once, from what is saved. Later pushes are this page's own saves.
  useEffect(() => {
    if (settings && overlay === null) {
      setLists(listsText(settings));
      setOverlay(settings.overlay);
    }
  }, [settings, overlay]);

  const sourceUrl = `${window.location.origin}/watchers`;

  async function save(event: FormEvent) {
    event.preventDefault();
    if (!overlay) return;
    const checks = [
      { entries: lines(lists.embeds), parse: parseChannel, what: 'an embed', how: 'platform/name, such as kick/destiny' },
      { entries: lines(lists.kickChats), parse: parseKickChat, what: 'a Kick channel', how: 'a channel name, such as destiny' },
      {
        entries: lines(lists.youtubeChats),
        parse: parseYouTubeChat,
        what: 'a YouTube chat',
        how: 'a handle such as @destiny, or a video id',
      },
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
      overlay,
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
    setLists(listsText((await response.json()) as Settings));
    setMessage({ text: 'Saved. Open overlays update straight away.', error: false });
  }

  async function quit() {
    await fetch('/api/quit', { method: 'POST', headers: { 'Content-Type': 'application/json' } }).catch(
      () => undefined,
    );
  }

  async function copy() {
    await navigator.clipboard.writeText(sourceUrl);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1_500);
  }

  if (!overlay) {
    return (
      <main className="setup">
        <p className="status">{chatStatus(snapshot, reachable)}</p>
      </main>
    );
  }

  return (
    <main className="setup">
      <h1>
        dgg-obs-tools <span className="version">{update?.current}</span>
      </h1>
      <p className="status">{chatStatus(snapshot, reachable)}</p>
      <UpdateNotice update={update} />

      <section>
        <h2>OBS browser source</h2>
        <div className="row">
          <input className="url" readOnly value={sourceUrl} onFocus={(event) => event.currentTarget.select()} />
          <button type="button" onClick={copy}>{copied ? 'Copied' : 'Copy'}</button>
        </div>
        <p className="hint">
          In OBS, add a Browser source with this URL, sized 1920 × 1080. It draws on a transparent
          background over whatever is below it. Keep this app running while you stream.
        </p>
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
          hint="A channel handle such as @destiny, which follows whatever that channel has live, or a video id or link. Everything above is drawn together in one overlay."
          onChange={(youtubeChats) => setLists({ ...lists, youtubeChats })}
        />

        <section>
          <h2>Overlay</h2>
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

        <div className="row">
          <button type="submit" className="primary">
            Save
          </button>
          {message && <p className={message.error ? 'message error' : 'message'}>{message.text}</p>}
        </div>
      </form>

      <section className="preview-section">
        <h2>Preview</h2>
        <Preview />
      </section>

      <section>
        <h2>Running</h2>
        <div className="row">
          <button type="button" onClick={quit}>
            Quit
          </button>
          <p className="hint inline">
            The app sits in the system tray (the menu bar on a Mac). Quitting stops the overlays in OBS.
          </p>
        </div>
      </section>
    </main>
  );
}

createRoot(document.getElementById('root')!).render(<SetupPage />);
