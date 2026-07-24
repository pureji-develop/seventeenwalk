/** Mahjong tile sprite helpers (public/tiles.webp + tiles.json). */

export const TILES_KEY = 'tiles';
export const TILES_PATH = '/tiles.webp';
export const TILES_META_PATH = '/tiles.json';

export const TILE_FRAME_W = 47;
export const TILE_FRAME_H = 63;

/** Display scale in GameScene. */
export const TILE_SCALE = 0.85;
export const TILE_DISPLAY_W = Math.round(TILE_FRAME_W * TILE_SCALE);
export const TILE_DISPLAY_H = Math.round(TILE_FRAME_H * TILE_SCALE);
export const TILE_GAP = 4;

export type TileMeta = {
  tileWidth: number;
  tileHeight: number;
  frames: { id: string; index: number }[];
};

const kindToFrame = new Map<string, number>();

export async function loadTileMeta(): Promise<void> {
  const res = await fetch(TILES_META_PATH);
  const meta = (await res.json()) as TileMeta;
  kindToFrame.clear();
  for (const f of meta.frames) {
    if (!kindToFrame.has(f.id)) {
      kindToFrame.set(f.id, f.index);
    }
  }
}

export function frameIndexForKind(kind: string): number {
  return kindToFrame.get(kind) ?? 0;
}

export function sortTilesByKind<T extends { kind: string; id: number }>(tiles: T[]): T[] {
  return [...tiles].sort((a, b) => {
    const fa = frameIndexForKind(a.kind);
    const fb = frameIndexForKind(b.kind);
    if (fa !== fb) return fa - fb;
    return a.id - b.id;
  });
}
