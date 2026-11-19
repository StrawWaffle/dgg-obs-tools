/**
 * Twitch and Kick let people pick any name colour, and some pick black, which
 * vanishes against a dark chat or a busy stream. A colour too dark to read is
 * lightened towards white until it can be, keeping its hue.
 */
const DARKEST = 0.35;

export function readableColor(color: string): string {
  const match = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(color);
  if (!match) return color;
  const [r, g, b] = match.slice(1).map((hex) => Number.parseInt(hex, 16) / 255);
  const lightness = 0.2126 * r + 0.7152 * g + 0.0722 * b;
  if (lightness >= DARKEST) return color;
  const towardsWhite = (DARKEST - lightness) / (1 - lightness);
  const lift = (channel: number) =>
    Math.round((channel + (1 - channel) * towardsWhite) * 255)
      .toString(16)
      .padStart(2, '0');
  return `#${lift(r)}${lift(g)}${lift(b)}`;
}
