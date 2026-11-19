import { useEffect } from 'react';

const EMOTE_STYLESHEET = 'https://r2cdn.destiny.gg/emotes/emotes.css';

/**
 * Points the page's emote stylesheet at the emote list the app last read.
 *
 * The CDN serves it with no Cache-Control, so an OBS browser source keeps
 * whatever copy it fetched, across restarts, and draws a new emote as nothing.
 * A changed list is a new URL. The link is changed in place rather than added
 * again, so the cascade order stays as it is.
 */
export function useEmoteStylesheetVersion(version: string | null): void {
  useEffect(() => {
    if (!version) return;
    const link = document.getElementById('emote-stylesheet');
    if (!(link instanceof HTMLLinkElement)) return;
    const href = `${EMOTE_STYLESHEET}?v=${version}`;
    if (link.href !== href) link.href = href;
  }, [version]);
}
