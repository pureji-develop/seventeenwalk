/** REST / WebSocket endpoint helpers. */

/** Shared local-dev room (backend keeps it alive). */
export const DEFAULT_ROOM_ID = 'room_test';

/** Vite development server (`npm run dev`), not production builds. */
export function isDevClient(): boolean {
  return import.meta.env.DEV === true;
}

export type CreateRoomOptions = {
  totalRounds?: number;
  /** Finite seconds only; omit or 0 → server default (apply infinite via set_options after join). */
  buildTimeoutSec?: number;
};

export async function createRoom(opts: CreateRoomOptions = {}): Promise<string> {
  const body: Record<string, number> = {};
  if (opts.totalRounds && opts.totalRounds > 0) {
    body.totalRounds = opts.totalRounds;
  }
  if (opts.buildTimeoutSec && opts.buildTimeoutSec > 0) {
    body.buildTimeoutSec = opts.buildTimeoutSec;
  }
  const res = await fetch('/rooms', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`ルーム作成に失敗しました (${res.status})`);
  }
  const data = (await res.json()) as { roomId: string };
  if (!data.roomId) {
    throw new Error('roomId がありません');
  }
  return data.roomId;
}

export interface PublicRoomInfo {
  roomId: string;
  hostName: string;
  players: number;
  phase: string;
  totalRounds: number;
  buildTimeoutSec: number;
  wallSorted: boolean;
  open: boolean;
  busy: boolean;
}

export async function fetchOpenRooms(): Promise<PublicRoomInfo[]> {
  const res = await fetch('/rooms');
  if (!res.ok) {
    throw new Error(`ルーム一覧の取得に失敗しました (${res.status})`);
  }
  const data = (await res.json()) as { rooms?: PublicRoomInfo[] };
  return Array.isArray(data.rooms) ? data.rooms : [];
}

export async function fetchRoomInfo(roomId: string): Promise<{ players: number; phase: string }> {
  const res = await fetch(`/rooms/${encodeURIComponent(roomId)}`);
  if (!res.ok) {
    throw new Error(`ルームが見つかりません (${res.status})`);
  }
  return res.json();
}

function playerIdStorageKey(roomId: string): string {
  return `sw:playerId:${roomId}`;
}

export function loadStoredPlayerId(roomId: string): string | null {
  try {
    return sessionStorage.getItem(playerIdStorageKey(roomId));
  } catch {
    return null;
  }
}

export function storePlayerId(roomId: string, playerId: string): void {
  try {
    sessionStorage.setItem(playerIdStorageKey(roomId), playerId);
  } catch {
    /* ignore */
  }
}

export function clearStoredPlayerId(roomId: string): void {
  try {
    sessionStorage.removeItem(playerIdStorageKey(roomId));
  } catch {
    /* ignore */
  }
}

/** Same-origin `/ws` (Vite proxy or nginx). */
export function buildGameWsUrl(roomId: string, name: string, playerId?: string | null): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const q = new URLSearchParams({ roomId, name });
  if (playerId) q.set('playerId', playerId);
  return `${protocol}//${window.location.host}/ws?${q.toString()}`;
}
