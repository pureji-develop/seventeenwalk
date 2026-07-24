import type { EventPayload, PublicState, ServerEnvelope, WelcomePayload } from '../types/protocol';
import {
  buildGameWsUrl,
  clearStoredPlayerId,
  loadStoredPlayerId,
  storePlayerId,
} from '../constants/network';

export type NetworkEventMap = {
  welcome: WelcomePayload;
  state: PublicState;
  event: EventPayload;
  waits: string[];
  error: string;
  /** Soft disconnect — reconnecting in progress. */
  reconnecting: string;
  /** Hard disconnect after reconnect exhausted or intentional leave. */
  disconnect: string;
};

type Handler<T> = (payload: T) => void;

const MAX_RECONNECT = 8;
const RECONNECT_BASE_MS = 600;

/**
 * WebSocket layer: connect / send / event fan-out / auto-reconnect.
 * Lobby may hand off an open socket to GameScene.
 */
export class NetworkManager {
  private socket: WebSocket | null = null;
  private roomId: string;
  private name: string;
  private playerId: string | null = null;
  private intentionalClose = false;
  private lastSeq = 0;
  private reconnectAttempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private handedOff = false;

  private listeners: { [K in keyof NetworkEventMap]: Handler<NetworkEventMap[K]>[] } = {
    welcome: [],
    state: [],
    event: [],
    waits: [],
    error: [],
    reconnecting: [],
    disconnect: [],
  };

  constructor(opts: { socket?: WebSocket; roomId: string; name: string; playerId?: string | null }) {
    this.roomId = opts.roomId;
    this.name = opts.name;
    this.playerId = opts.playerId ?? loadStoredPlayerId(opts.roomId);
    if (opts.socket) {
      this.socket = opts.socket;
      this.bindSocket(opts.socket);
    }
  }

  connect(): void {
    this.intentionalClose = false;
    this.handedOff = false;
    this.clearReconnectTimer();
    const url = buildGameWsUrl(this.roomId, this.name, this.playerId);
    const socket = new WebSocket(url);
    this.socket = socket;
    this.bindSocket(socket);
  }

  /** Detach handlers without closing — for Lobby → Game handoff. */
  detach(): WebSocket | null {
    this.handedOff = true;
    this.clearReconnectTimer();
    const s = this.socket;
    if (s) {
      s.onmessage = null;
      s.onopen = null;
      s.onclose = null;
      s.onerror = null;
    }
    this.socket = null;
    return s;
  }

  /** Player id from welcome (for GameScene handoff). */
  getPlayerId(): string | null {
    return this.playerId;
  }

  close(): void {
    this.intentionalClose = true;
    this.clearReconnectTimer();
    clearStoredPlayerId(this.roomId);
    this.socket?.close();
    this.socket = null;
  }

  on<K extends keyof NetworkEventMap>(event: K, handler: Handler<NetworkEventMap[K]>): void {
    this.listeners[event].push(handler);
  }

  off<K extends keyof NetworkEventMap>(event: K, handler: Handler<NetworkEventMap[K]>): void {
    const list = this.listeners[event];
    const idx = list.indexOf(handler);
    if (idx >= 0) list.splice(idx, 1);
  }

  /** Returns false if the socket is not open (and emits error). */
  send(type: string, payload: Record<string, unknown> = {}): boolean {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      this.emit('error', '送信できません（未接続）。再接続を待つか、操作をやり直してください');
      return false;
    }
    this.socket.send(JSON.stringify({ type, payload }));
    return true;
  }

  get isOpen(): boolean {
    return this.socket?.readyState === WebSocket.OPEN;
  }

  private emit<K extends keyof NetworkEventMap>(event: K, payload: NetworkEventMap[K]): void {
    this.listeners[event].forEach((h) => h(payload));
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer != null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private scheduleReconnect() {
    if (this.intentionalClose || this.handedOff) return;
    if (this.reconnectAttempt >= MAX_RECONNECT) {
      this.emit('disconnect', '再接続に失敗しました');
      return;
    }
    const attempt = this.reconnectAttempt++;
    const delay = Math.min(8000, RECONNECT_BASE_MS * Math.pow(1.6, attempt));
    this.emit('reconnecting', `再接続中…（${attempt + 1}/${MAX_RECONNECT}）`);
    this.clearReconnectTimer();
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (this.intentionalClose || this.handedOff) return;
      this.connect();
    }, delay);
  }

  private bindSocket(socket: WebSocket) {
    socket.onopen = () => {
      this.reconnectAttempt = 0;
    };

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data as string) as ServerEnvelope;
        if (
          (message.type === 'state' || message.type === 'event') &&
          typeof message.seq === 'number'
        ) {
          if (message.seq < this.lastSeq) return;
          this.lastSeq = message.seq;
        }
        switch (message.type) {
          case 'welcome': {
            const welcome = message.payload as WelcomePayload;
            if (welcome?.playerId) {
              this.playerId = welcome.playerId;
              storePlayerId(this.roomId, welcome.playerId);
            }
            this.emit('welcome', welcome);
            break;
          }
          case 'state':
            this.emit('state', message.payload as PublicState);
            break;
          case 'event':
            this.emit('event', message.payload as EventPayload);
            break;
          case 'waits': {
            const p = message.payload as { waits?: unknown } | null;
            const raw = p && typeof p === 'object' ? p.waits : null;
            this.emit(
              'waits',
              Array.isArray(raw) ? raw.filter((x): x is string => typeof x === 'string') : []
            );
            break;
          }
          case 'error': {
            const err = (message.payload ?? {}) as { message?: string; code?: string };
            this.emit('error', err.message || err.code || 'Unknown error');
            break;
          }
          case 'pong':
            break;
          default:
            console.log('Unhandled WS message', message);
        }
      } catch (err) {
        console.error('WS parse error', err);
      }
    };

    socket.onclose = () => {
      if (this.intentionalClose || this.handedOff) return;
      this.socket = null;
      this.scheduleReconnect();
    };

    socket.onerror = () => {
      // onclose handles reconnect UX
    };
  }
}
