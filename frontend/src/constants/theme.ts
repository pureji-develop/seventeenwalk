/** Shared UI theme tokens — mahjong felt (濃緑マット) look. */

export const FONT_DISPLAY = '"Shippori Mincho", "Hiragino Mincho ProN", serif';
export const FONT_UI = '"Source Sans 3", "Segoe UI", sans-serif';

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
