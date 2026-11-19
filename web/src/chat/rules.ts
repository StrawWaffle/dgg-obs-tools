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
