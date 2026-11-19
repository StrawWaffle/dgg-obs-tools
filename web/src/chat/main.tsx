import '@fontsource/roboto/400.css';
import '@fontsource/roboto/700.css';
import { createRoot } from 'react-dom/client';
import { useEmoteStylesheetVersion } from '../emoteStylesheet';
import '../watchers/flairs.css';
import ChatView from './ChatView';
import './chat.css';
import { useChatEvents } from './useChatEvents';

function ChatPage() {
  const { settings, messages, emoteVersion, reachable } = useChatEvents();
  useEmoteStylesheetVersion(emoteVersion);
  if (!settings) {
    return reachable ? null : <p className="chat-waiting">Waiting for dgg-obs-tools to start…</p>;
  }
  return <ChatView messages={messages} options={settings.chat} />;
}

createRoot(document.getElementById('root')!).render(<ChatPage />);
