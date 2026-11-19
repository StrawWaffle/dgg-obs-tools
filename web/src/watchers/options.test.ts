import { describe, expect, it } from 'vitest';
import { parseChannel, parseKickChat, parseTwitchChat, parseYouTubeChat } from './options';

describe('reading an embed out of what somebody pastes', () => {
  it('takes platform/name, lowercased', () => {
    expect(parseChannel('kick/Destiny')).toEqual({ platform: 'kick', id: 'destiny' });
    expect(parseChannel('youtube/@Destiny')).toEqual({ platform: 'youtube', id: '@destiny' });
  });

  it('keeps a YouTube video id as it is, because its case matters', () => {
    expect(parseChannel('youtube/P7Xv5DdqL0k')).toEqual({ platform: 'youtube', id: 'P7Xv5DdqL0k' });
  });

  it('reads destiny.gg bigscreen links', () => {
    expect(parseChannel('https://www.destiny.gg/bigscreen#kick/dggJams')).toEqual({ platform: 'kick', id: 'dggjams' });
  });

  it('reads Kick and YouTube links', () => {
    expect(parseChannel('https://kick.com/Destiny')).toEqual({ platform: 'kick', id: 'destiny' });
    expect(parseChannel('https://www.youtube.com/@destiny/live')).toEqual({ platform: 'youtube', id: '@destiny' });
    expect(parseChannel('https://www.youtube.com/watch?v=P7Xv5DdqL0k')).toEqual({ platform: 'youtube', id: 'P7Xv5DdqL0k' });
    expect(parseChannel('https://youtu.be/P7Xv5DdqL0k')).toEqual({ platform: 'youtube', id: 'P7Xv5DdqL0k' });
  });

  it('refuses what it cannot place', () => {
    expect(parseChannel('destiny')).toBeNull();
    expect(parseChannel('vimeo/123')).toBeNull();
  });
});

describe('reading a Kick or YouTube chat', () => {
  it('takes a Kick name, with or without the platform, or a link', () => {
    expect(parseKickChat('Destiny')).toBe('destiny');
    expect(parseKickChat('kick/destiny')).toBe('destiny');
    expect(parseKickChat('https://kick.com/destiny')).toBe('destiny');
    expect(parseKickChat('https://www.youtube.com/@destiny')).toBeNull();
  });

  it('takes a YouTube handle, a video id, or a link', () => {
    expect(parseYouTubeChat('@Destiny')).toBe('@destiny');
    expect(parseYouTubeChat('youtube/@destiny')).toBe('@destiny');
    expect(parseYouTubeChat('P7Xv5DdqL0k')).toBe('P7Xv5DdqL0k');
    expect(parseYouTubeChat('https://www.youtube.com/watch?v=P7Xv5DdqL0k')).toBe('P7Xv5DdqL0k');
    expect(parseYouTubeChat('destiny')).toBeNull();
  });
});

describe('reading a Twitch chat', () => {
  it('takes a name, with or without the platform, or a link', () => {
    expect(parseTwitchChat('xQc')).toBe('xqc');
    expect(parseTwitchChat('twitch/xqc')).toBe('xqc');
    expect(parseTwitchChat('https://www.twitch.tv/xqc')).toBe('xqc');
    expect(parseTwitchChat('https://kick.com/xqc')).toBeNull();
    expect(parseTwitchChat('x')).toBeNull();
  });
});
