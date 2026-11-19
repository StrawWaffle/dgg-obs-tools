import '@fontsource/share-tech-mono';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { createRoot } from 'react-dom/client';
import { useAppEvents } from '../useAppEvents';
import {
  LABELS,
  NUMBER_RANGES,
  parseChannel,
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
];

const NUMBERS: { name: NumberField; label: string; hint: string }[] = [
  { name: 'window', label: 'Minutes', hint: 'How recently somebody must have spoken, for recent speakers.' },
  { name: 'max', label: 'At most', hint: 'Twelve suits a 1920 × 1080 source.' },
  { name: 'size', label: 'Size %', hint: 'Emote and name together.' },
  { name: 'speed', label: 'Speed %', hint: '200 is twice as quick, 25 a crawl.' },
  { name: 'roam', label: 'Roam %', hint: 'How far everybody wanders. 0 holds them still.' },
  { name: 'inset', label: 'Edge inset %', hint: 'Kept clear at every edge so nothing is cropped.' },
];

function channelText(settings: Settings): string {
  return settings.channel ? `${settings.channel.platform}/${settings.channel.id}` : '';
}

function chatStatus(snapshot: WatchersSnapshot | null, reachable: boolean): string {
  if (!reachable) return 'The app is not running. Start it again and this page reconnects.';
  if (!snapshot?.channel) return 'Choose an embed to start.';
  const where = `${snapshot.channel.platform}/${snapshot.channel.id}`;
  if (!snapshot.connected) return `Connecting to destiny.gg chat for ${where}…`;
  const people = snapshot.chatCount === 1 ? '1 person' : `${snapshot.chatCount} people`;
  return `Connected. ${people} in chat on ${where}.`;
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
  const [channel, setChannel] = useState('');
  const [overlay, setOverlay] = useState<OverlayOptions | null>(null);
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);
  const [copied, setCopied] = useState(false);

  // Fill the form once, from what is saved. Later pushes are this page's own saves.
  useEffect(() => {
    if (settings && overlay === null) {
      setChannel(channelText(settings));
      setOverlay(settings.overlay);
    }
  }, [settings, overlay]);

  const sourceUrl = `${window.location.origin}/watchers`;

  async function save(event: FormEvent) {
    event.preventDefault();
    if (!overlay) return;
    const trimmed = channel.trim();
    const parsed = trimmed === '' ? null : parseChannel(trimmed);
    if (trimmed !== '' && !parsed) {
      setMessage({
        text: 'Write the embed as platform/name, such as kick/destiny, or paste a destiny.gg bigscreen link.',
        error: true,
      });
      return;
    }
    const response = await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ channel: parsed, overlay } satisfies Settings),
    }).catch(() => null);
    if (!response?.ok) {
      setMessage({ text: response ? await response.text() : 'The app is not answering.', error: true });
      return;
    }
    const saved = (await response.json()) as Settings;
    setChannel(channelText(saved));
    setMessage({ text: 'Saved. Open overlays update straight away.', error: false });
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
      <h1>dgg-obs-tools</h1>
      <p className="status">{chatStatus(snapshot, reachable)}</p>

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
        <section>
          <h2>Embed</h2>
          <label>
            Whose watchers to draw
            <input
              value={channel}
              placeholder="kick/destiny"
              onChange={(event) => setChannel(event.currentTarget.value)}
            />
          </label>
          <p className="hint">
            platform/name, such as kick/destiny or youtube/VIDEO_ID, or a link like
            https://www.destiny.gg/bigscreen#kick/destiny.
          </p>
        </section>

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
    </main>
  );
}

createRoot(document.getElementById('root')!).render(<SetupPage />);
