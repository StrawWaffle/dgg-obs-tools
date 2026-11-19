import { useEffect, useState } from 'react';
import type { Settings, WatchersSnapshot } from './watchers/options';

export interface AppEvents {
  settings: Settings | null;
  snapshot: WatchersSnapshot | null;
  /** Whether the app itself is answering. False while it is closed or restarting. */
  reachable: boolean;
}

/**
 * The app's settings and the watchers snapshot, kept current over server-sent
 * events. The app sends both as soon as a page connects, and the browser
 * reconnects by itself if the app restarts, after an update say.
 */
export function useAppEvents(): AppEvents {
  const [state, setState] = useState<AppEvents>({ settings: null, snapshot: null, reachable: false });

  useEffect(() => {
    const source = new EventSource('/api/events');
    source.addEventListener('open', () => setState((current) => ({ ...current, reachable: true })));
    source.addEventListener('error', () => setState((current) => ({ ...current, reachable: false })));
    source.addEventListener('settings', (event) => {
      const settings = JSON.parse((event as MessageEvent<string>).data) as Settings;
      setState((current) => ({ ...current, settings, reachable: true }));
    });
    source.addEventListener('snapshot', (event) => {
      const snapshot = JSON.parse((event as MessageEvent<string>).data) as WatchersSnapshot;
      setState((current) => ({ ...current, snapshot, reachable: true }));
    });
    return () => source.close();
  }, []);

  return state;
}
