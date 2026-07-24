/** REST / WebSocket endpoint helpers. */

/** Shared local-dev room (backend keeps it alive). */
export const DEFAULT_ROOM_ID = 'room_test';

/** Vite development server (`npm run dev`), not production builds. */
export function isDevClient(): boolean {
  return import.meta.env.DEV === true;
}

export async function createRoom(): Promise<string> {
  const res = await fetch('/rooms', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
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
