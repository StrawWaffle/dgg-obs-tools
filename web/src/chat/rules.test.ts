import { describe, expect, it } from 'vitest';
import type { ChatMessage, ChatOptions } from '../watchers/options';
import { highlighted, linkText, shown, visible } from './rules';

const OPTIONS: ChatOptions = {
  dgg: 'all',
  kick: true,
  youtube: true,
  size: 100,
  shadow: 'soft',
  icons: 'on',
  highlights: [],
  highlightWatching: 'on',
  links: 'short',
  removed: 'hide',
  lines: 150,
};

function message(over: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id: 'm',
    platform: 'dgg',
    source: 'destiny.gg',
    nick: 'Anpan',
    flair: null,
    color: null,
    at: '2026-11-19T10:00:00Z',
    parts: [{ text: 'hello there' }],
    ...over,
  };
}

describe('which messages show', () => {
  it('follows the destiny.gg setting', () => {
    expect(shown(message(), OPTIONS)).toBe(true);
    expect(shown(message(), { ...OPTIONS, dgg: 'watching' })).toBe(false);
    expect(shown(message({ watching: true }), { ...OPTIONS, dgg: 'watching' })).toBe(true);
    expect(shown(message({ watching: true }), { ...OPTIONS, dgg: 'off' })).toBe(false);
  });

  it('follows the Kick and YouTube switches', () => {
    expect(shown(message({ platform: 'kick' }), { ...OPTIONS, kick: false })).toBe(false);
    expect(shown(message({ platform: 'youtube' }), OPTIONS)).toBe(true);
  });

  it('hides removed messages only when asked to', () => {
    expect(shown(message({ removed: true }), OPTIONS)).toBe(false);
    expect(shown(message({ removed: true }), { ...OPTIONS, removed: 'blur' })).toBe(true);
  });

  it('keeps the newest lines', () => {
    const many = Array.from({ length: 30 }, (_, i) => message({ id: String(i) }));
    const kept = visible(many, { ...OPTIONS, lines: 20 });
    expect(kept).toHaveLength(20);
    expect(kept.at(-1)?.id).toBe('29');
  });
});

describe('highlighting', () => {
  it('matches phrases without case, in the name or the message', () => {
    expect(highlighted(message(), { ...OPTIONS, highlights: ['THERE'] })).toBe(true);
    expect(highlighted(message(), { ...OPTIONS, highlights: ['anpan'] })).toBe(true);
    expect(highlighted(message(), { ...OPTIONS, highlights: ['nope'] })).toBe(false);
  });

  it('marks people watching a followed embed when asked', () => {
    expect(highlighted(message({ watching: true }), OPTIONS)).toBe(true);
    expect(highlighted(message({ watching: true }), { ...OPTIONS, highlightWatching: 'off' })).toBe(false);
  });
});

describe('links', () => {
  it('shortens long ones to their site', () => {
    expect(linkText('https://www.example.com/a/very/long/path/that/goes/on', 'short')).toBe(
      'example.com/a/very/long/pa…',
    );
    expect(linkText('https://x.y/a', 'short')).toBe('https://x.y/a');
    expect(linkText('https://www.example.com/a/very/long/path/that/goes/on', 'full')).toContain('https://');
  });
});
