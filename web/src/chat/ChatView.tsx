import { useLayoutEffect, useRef, useState } from 'react';
import { readableColor } from '../color';
import { PlatformIcon } from '../platform';
import type { ChatMessage, ChatOptions, MessagePart } from '../watchers/options';
import { censor, highlighted, linkText, visible } from './rules';

/** OBS's browser source defines this; an ordinary browser does not. */
export const inOBS = typeof window !== 'undefined' && 'obsstudio' in window;

/** How close to the bottom still counts as reading the newest messages. */
const STICK_PX = 48;

function Part({ part, options }: { part: MessagePart; options: ChatOptions }) {
  if (part.emote && part.url) {
    return <img className="emote-picture" src={part.url} alt={part.emote} title={part.emote} />;
  }
  if (part.emote) {
    return (
      <span className={`emote ${part.emote}`} title={part.emote}>
        {part.emote}
      </span>
    );
  }
  if (part.link) {
    return (
      <a href={part.link} target="_blank" rel="noopener noreferrer">
        {linkText(part.link, options.links)}
      </a>
    );
  }
  return <>{options.censor === 'on' && part.text ? censor(part.text) : part.text}</>;
}

function Line({ message, options }: { message: ChatMessage; options: ChatOptions }) {
  const name =
    message.flair && message.platform === 'dgg' ? `chat-name flair-${message.flair}` : 'chat-name';
  const classes = [
    'chat-line',
    highlighted(message, options) ? 'chat-highlight' : '',
    message.removed && options.removed === 'blur' ? 'chat-removed' : '',
    message.action ? 'chat-action' : '',
    message.green ? 'chat-green' : '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div className={classes}>
      {options.icons === 'on' && <PlatformIcon platform={message.platform} />}
      {options.badges === 'on' &&
        message.badges?.map((badge) => (
          <img key={badge.url} className="chat-badge" src={badge.url} alt={badge.label} title={badge.label} />
        ))}
      <span className={name} style={message.color ? { color: readableColor(message.color) } : undefined}>
        {message.nick}
      </span>
      {!message.action && <span className="chat-colon">: </span>}
      {/* `text` is the container chat-gui puts a message's emotes in, and parts
          of destiny.gg's stylesheet are written against it. */}
      <span className="text">
        {message.parts.map((part, index) => (
          <Part key={index} part={part} options={options} />
        ))}
      </span>
    </div>
  );
}

export default function ChatView({ messages, options }: { messages: ChatMessage[]; options: ChatOptions }) {
  const list = useRef<HTMLDivElement>(null);
  // Whether the reader is at the newest messages. OBS has no reader, so it always is.
  const [following, setFollowing] = useState(true);
  // When the reader last scrolled by hand. Only that stops the chat following:
  // a window resize or a burst of messages also fires scroll events, and once
  // read those as somebody reading back.
  const handled = useRef(0);
  const lines = visible(messages, options);

  useLayoutEffect(() => {
    const element = list.current;
    if (element && (following || inOBS)) element.scrollTop = element.scrollHeight;
  });

  function byHand() {
    handled.current = Date.now();
  }

  function onScroll() {
    const element = list.current;
    if (!element || inOBS) return;
    const atBottom = element.scrollHeight - element.scrollTop - element.clientHeight < STICK_PX;
    if (atBottom) setFollowing(true);
    else if (Date.now() - handled.current < 1_000) setFollowing(false);
  }

  return (
    <div
      className={`chat chat-shadow-${options.shadow} ${inOBS ? 'chat-obs' : 'chat-browser'}`}
      style={{ '--chat-size': String(options.size / 100) } as React.CSSProperties}
    >
      <div
        className="chat-lines"
        ref={list}
        onScroll={onScroll}
        onWheel={byHand}
        onTouchMove={byHand}
        onKeyDown={byHand}
        onPointerDown={byHand}
        tabIndex={-1}
      >
        {lines.map((message) => (
          <Line key={message.id} message={message} options={options} />
        ))}
      </div>
      {!following && !inOBS && (
        <button type="button" className="chat-more" onClick={() => setFollowing(true)}>
          Newer messages below
        </button>
      )}
    </div>
  );
}
