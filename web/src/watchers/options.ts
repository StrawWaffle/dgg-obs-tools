/**
 * How the watchers overlay draws, and what it draws from. The app saves the
 * options in its settings (internal/settings/settings.go, which has the same
 * shape and the same limits) and sends both over /api/events.
 */

export const platforms = ['kick', 'youtube', 'twitch', 'angelthump'] as const;
export type Platform = (typeof platforms)[number];

export interface Channel {
  platform: Platform;
  /** Lowercase, as chat reports it. */
  id: string;
}

/**
 * One person watching a followed embed: somebody in destiny.gg chat with it
 * open, or somebody talking in its own Kick or YouTube chat.
 */
export interface Watcher {
  nick: string;
  platform: 'dgg' | 'kick' | 'youtube';
  /** The destiny.gg flair their name is coloured by, resolved the way chat resolves it. */
  flair: string | null;
  /** A name colour the platform gives directly (Kick). */
  color: string | null;
  /** When they last said something. */
  lastSpokeAt: string | null;
  /** The last emote they used. A destiny.gg one is drawn by this name. */
  lastEmote: string | null;
  /** The picture of a Kick or YouTube emote. */
  emoteUrl: string | null;
}

export interface WatchersSnapshot {
  /** The embeds followed, drawn together. */
  channels: Channel[];
  /** For each followed YouTube channel, the live stream found on it, or "". */
  liveVideos: Record<string, string>;
  /** Everyone on them, which may be more than `watchers` carries. */
  chatCount: number;
  /** Whoever spoke most recently first. */
  watchers: Watcher[];
  /** The emote list's version, for the stylesheet URL. Null until it is read. */
  emoteVersion: string | null;
  connected: boolean;
}

export const watcherShows = ['speakers', 'all'] as const;
export const watcherLayouts = ['float', 'safe', 'rail', 'column', 'sides', 'climb', 'bump'] as const;
export type WatcherLayout = (typeof watcherLayouts)[number];
/** The path a watcher traces. `bump` is a simulation rather than a path, so it ignores this. */
export const watcherMotions = ['drift', 'bob', 'orbit', 'sway'] as const;
export const watcherNames = ['under', 'beside', 'off'] as const;
export const watcherEntrances = ['fade', 'spin', 'slide', 'random'] as const;
export type WatcherEntrance = (typeof watcherEntrances)[number];
/**
 * Chat's colours mean something to somebody reading chat; over a stream they
 * can read as decoration, and two of them are moving gradients.
 */
export const watcherColors = ['flair', 'white'] as const;

export interface OverlayOptions {
  show: (typeof watcherShows)[number];
  /** Minutes, for `show=speakers`. */
  window: number;
  max: number;
  layout: WatcherLayout;
  names: (typeof watcherNames)[number];
  enter: WatcherEntrance;
  motion: (typeof watcherMotions)[number];
  color: (typeof watcherColors)[number];
  /** Percent of the standard pace. 200 is twice as fast, 25 a crawl. */
  speed: number;
  /** Percent of the size the layout draws at, emote and name together. */
  size: number;
  /** Percent of the standard distance travelled. 0 holds everybody still. */
  roam: number;
  /** Percent of the frame kept clear at every edge, so nothing is cropped. */
  inset: number;
  /** A small K or YT before Kick and YouTube names. */
  marks: 'on' | 'off';
}

/** How many embeds one overlay can follow; the app enforces the same limit. */
export const MAX_CHANNELS = 10;

export interface Settings {
  channels: Channel[];
  /** Whether a Kick or YouTube embed brings the people talking in that stream's own chat. */
  platformChats: boolean;
  overlay: OverlayOptions;
}

export const DEFAULT_OVERLAY_OPTIONS: OverlayOptions = {
  show: 'speakers',
  window: 10,
  max: 12,
  layout: 'float',
  names: 'under',
  enter: 'fade',
  motion: 'drift',
  color: 'flair',
  speed: 100,
  size: 100,
  roam: 100,
  inset: 4,
  marks: 'on',
};

/** Stands in for anyone who has not used an emote yet. */
export const DEFAULT_EMOTE = 'MMMM';

export const NUMBER_RANGES = {
  window: { min: 1, max: 1_440 },
  max: { min: 1, max: 100 },
  speed: { min: 10, max: 400 },
  size: { min: 25, max: 400 },
  roam: { min: 0, max: 300 },
  inset: { min: 0, max: 25 },
} as const;

export const LABELS = {
  show: { speakers: 'Recent speakers', all: 'Everyone' },
  layout: {
    float: 'Free float',
    safe: 'Safe frame',
    rail: 'Bottom wander',
    column: 'Side climb',
    sides: 'Twin sides',
    climb: 'Twin-side climb',
    bump: 'Bumper cars',
  },
  names: { under: 'Under emote', beside: 'Beside emote', off: 'Hidden' },
  enter: { fade: 'Fade', spin: 'Spin', slide: 'Slide', random: 'Mixed' },
  motion: { drift: 'Wander', bob: 'Bob up and down', orbit: 'Small circles', sway: 'Sway side to side' },
  color: { flair: 'Chat colours', white: 'Plain white' },
  marks: { on: 'K / YT marks', off: 'No marks' },
} as const;

/**
 * Reads an embed out of whatever somebody pastes: `kick/destiny`,
 * `youtube/@destiny`, `youtube/VIDEO_ID`, a destiny.gg bigscreen link such as
 * `https://www.destiny.gg/bigscreen#kick/destiny`, or a Kick or YouTube link.
 * A YouTube video id keeps its case, which matters; everything else is
 * lowercased, as chat reports it.
 */
export function parseChannel(input: string): Channel | null {
  const text = input.trim();
  const fromLink = channelFromLink(text);
  if (fromLink) return fromLink;

  const fragment = text.includes('#') ? text.slice(text.indexOf('#') + 1) : text;
  const [platform, ...rest] = fragment.split('/');
  const id = rest.join('/').trim();
  const known = platforms.find((value) => value === platform.trim().toLowerCase());
  if (!known || !id) return null;
  return { platform: known, id: known === 'youtube' && VIDEO_ID.test(id) ? id : id.toLowerCase() };
}

const VIDEO_ID = /^[\w-]{11}$/;

function channelFromLink(text: string): Channel | null {
  let url: URL;
  try {
    url = new URL(text);
  } catch {
    return null;
  }
  const host = url.hostname.replace(/^www\./, '').replace(/^m\./, '');
  const path = url.pathname.split('/').filter(Boolean);
  if (host === 'kick.com' && path[0]) return { platform: 'kick', id: path[0].toLowerCase() };
  if (host === 'youtu.be' && path[0] && VIDEO_ID.test(path[0])) return { platform: 'youtube', id: path[0] };
  if (host === 'youtube.com') {
    const video = url.searchParams.get('v') ?? (path[0] === 'live' ? path[1] : null);
    if (video && VIDEO_ID.test(video)) return { platform: 'youtube', id: video };
    if (path[0]?.startsWith('@')) return { platform: 'youtube', id: path[0].toLowerCase() };
  }
  return null;
}
