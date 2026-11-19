import { useEffect, useState } from 'react';
import type { ChatMessage, Settings } from '../watchers/options';

/** Matches the app's own history limit; nothing past it can be shown. */
const KEEP = 300;

export interface ChatEvents {
  settings: Settings | null;
  messages: ChatMessage[];
  emoteVersion: string | null;
  reachable: boolean;
}

/**
 * The combined chat, kept current over server-sent events: the recent history
 * when the page connects, then each message and each removal as it happens.
 * The browser reconnects by itself, and a reconnect starts again from history.
 */
export function useChatEvents(): ChatEvents {
  const [state, setState] = useState<ChatEvents>({
    settings: null,
    messages: [],
    emoteVersion: null,
    reachable: false,
  });

  useEffect(() => {
    const source = new EventSource('/api/chat/events');
    const read = <T,>(event: Event) => JSON.parse((event as MessageEvent<string>).data) as T;

    source.addEventListener('open', () => setState((current) => ({ ...current, reachable: true })));
    source.addEventListener('error', () => setState((current) => ({ ...current, reachable: false })));
    source.addEventListener('settings', (event) => {
      const settings = read<Settings>(event);
      setState((current) => ({ ...current, settings }));
    });
    source.addEventListener('emotes', (event) => {
      const emoteVersion = read<string | null>(event);
      setState((current) => ({ ...current, emoteVersion }));
    });
    source.addEventListener('history', (event) => {
      const messages = read<ChatMessage[] | null>(event) ?? [];
      setState((current) => ({ ...current, messages, reachable: true }));
    });
    source.addEventListener('message', (event) => {
      const message = read<ChatMessage>(event);
      setState((current) => {
        const messages = [...current.messages, message];
        return { ...current, messages: messages.length > KEEP ? messages.slice(-KEEP) : messages };
      });
    });
    source.addEventListener('remove', (event) => {
      const ids = new Set(read<{ ids: string[] }>(event).ids);
      setState((current) => ({
        ...current,
        messages: current.messages.map((message) => (ids.has(message.id) ? { ...message, removed: true } : message)),
      }));
    });
    return () => source.close();
  }, []);

  return state;
}
