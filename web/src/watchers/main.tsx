import '@fontsource/share-tech-mono';
import { createRoot } from 'react-dom/client';
import { useAppEvents } from '../useAppEvents';
import WatchersOverlay from './WatchersOverlay';

function OverlayPage() {
  const { settings, snapshot } = useAppEvents();
  // Nothing is drawn until the saved options arrive, so a source never flashes
  // the defaults before switching to what was chosen.
  if (!settings) return null;
  return <WatchersOverlay snapshot={snapshot} options={settings.overlay} />;
}

createRoot(document.getElementById('root')!).render(<OverlayPage />);
