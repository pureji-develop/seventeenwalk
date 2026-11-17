/** Shared UI theme tokens — mahjong felt (濃緑マット) look. */

export const FONT_DISPLAY = '"Shippori Mincho", "Hiragino Mincho ProN", serif';
export const FONT_UI = '"Source Sans 3", "Segoe UI", sans-serif';

/** Phaser / in-game text ≈2×. Layout offsets use a milder factor to avoid overlap. */
export const UI_TEXT_SCALE = 2;
export const UI_SPACE_SCALE = 1.4;

/** Tile display multiplier on top of the base TILE_SCALE. */
export const TILE_SIZE_SCALE = 1.3;

export function uiFont(px: number): string {
  return `${Math.round(px * UI_TEXT_SCALE)}px`;
}

/** Vertical/horizontal spacing that grows with larger UI, but less than full text scale. */
export function uiSpace(px: number): number {
  return Math.round(px * UI_SPACE_SCALE);
}

export const UI_COLOR = {
  table: '#0c2e24',
  tableDeep: '#071f18',
  ivory: '#e8eef4',
  accent: '#e25b4a',
  accentSoft: '#f0a399',
  mint: '#6ec9b0',
  muted: '#8a9f94',
  ink: '#0c2e24',
  panel: '#143a30',
  gold: '#d4b06a',
} as const;

/** Phaser fill colors (0xRRGGBB) matching UI_COLOR. */
export const PHASER_COLOR = {
  table: 0x0c2e24,
  tableDeep: 0x071f18,
  panel: 0x143a30,
  dim: 0x051a14,
  slot: 0x143a30,
} as const;

export const GAME_WIDTH = 1280;
export const GAME_HEIGHT = 720;
