/**
 * The small mark before a name that says which chat somebody is in. The Kick
 * and YouTube shapes are their own logos; destiny.gg's is its chat icon.
 */
export function PlatformIcon({ platform }: { platform: string }) {
  if (platform === 'kick') {
    return (
      <svg className="platform-icon" viewBox="0 0 24 24" fill="#53FC19" role="img" aria-label="Kick">
        <path d="M1.333 0h8v5.333H12V2.667h2.667V0h8v8H20v2.667h-2.667v2.666H20V16h2.667v8h-8v-2.667H12v-2.666H9.333V24h-8Z" />
      </svg>
    );
  }
  if (platform === 'youtube') {
    return (
      <svg className="platform-icon" viewBox="0 0 24 24" fill="#FF0000" role="img" aria-label="YouTube">
        <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
      </svg>
    );
  }
  return <img className="platform-icon" src="/platform/dgg.png" alt="destiny.gg" />;
}
