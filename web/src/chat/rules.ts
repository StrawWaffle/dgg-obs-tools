import type { ChatMessage, ChatOptions } from '../watchers/options';

/** Whether the options let a message show at all. */
export function shown(message: ChatMessage, options: ChatOptions): boolean {
  if (message.removed && options.removed === 'hide') return false;
  switch (message.platform) {
    case 'dgg':
      if (options.dgg === 'off') return false;
      return options.dgg === 'all' || Boolean(message.watching);
    case 'kick':
      return options.kick;
    case 'youtube':
      return options.youtube;
    case 'twitch':
      return options.twitch;
  }
}

/** The messages to draw, newest last, at most `lines` of them. */
export function visible(messages: ChatMessage[], options: ChatOptions): ChatMessage[] {
  const kept = messages.filter((message) => shown(message, options));
  return kept.slice(Math.max(0, kept.length - options.lines));
}

/** What a message says, emotes by name, for matching highlighted phrases. */
function plainText(message: ChatMessage): string {
  return message.parts.map((part) => part.text ?? part.emote ?? part.link ?? '').join('');
}

/**
 * Whether a message stands out: it mentions a highlighted phrase, or its
 * destiny.gg sender is watching a followed embed and the options ask for that.
 */
export function highlighted(message: ChatMessage, options: ChatOptions): boolean {
  if (options.highlightWatching === 'on' && message.watching) return true;
  if (options.highlights.length === 0) return false;
  const text = `${message.nick} ${plainText(message)}`.toLowerCase();
  return options.highlights.some((phrase) => text.includes(phrase.toLowerCase()));
}

/** A link as shown: in full, or as its site and the start of its path. */
export function linkText(link: string, style: ChatOptions['links']): string {
  if (style === 'full' || link.length <= 40) return link;
  try {
    const url = new URL(link);
    const rest = `${url.pathname}${url.search}`;
    return `${url.hostname.replace(/^www\./, '')}${rest.length > 16 ? `${rest.slice(0, 15)}…` : rest}`;
  } catch {
    return `${link.slice(0, 39)}…`;
  }
}

/**
 * Swear words and slurs. A stem catches every word that starts with it; the
 * short ones are matched only as whole words, so "spicy" and "cockpit" stay.
 */
const STEMS = ['fuck', 'shit', 'cunt', 'bitch', 'bastard', 'asshole', 'nigg', 'fagg', 'retard', 'whore', 'slut', 'wank', 'tranny', 'trannies'];
const WORDS = ['fag', 'fags', 'dick', 'dicks', 'cock', 'cocks', 'pussy', 'twat', 'twats', 'kike', 'kikes', 'spic', 'spics', 'chink', 'chinks', 'kys', 'cum'];
const SWEARING = new RegExp(`\\b(?:(?:${STEMS.join('|')})[a-z]*|(?:${WORDS.join('|')}))\\b`, 'gi');

/** Text with every swear word starred out after its first letter. */
export function censor(text: string): string {
  return text.replace(SWEARING, (word) => word[0] + '*'.repeat(word.length - 1));
}

/**
 * Whether a message carries on from the one drawn above it: the same person,
 * on the same platform, so their name need not be said again. Only the lines
 * actually drawn count, so a hidden message in between does not split a run.
 */
export function continues(previous: ChatMessage | undefined, message: ChatMessage): boolean {
  return (
    previous !== undefined &&
    previous.platform === message.platform &&
    previous.nick.toLowerCase() === message.nick.toLowerCase()
  );
}
