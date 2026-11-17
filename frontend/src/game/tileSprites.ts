import * as Phaser from 'phaser';
import type { TileView } from '../types/protocol';
import { FONT_UI, TILE_SIZE_SCALE, UI_COLOR, uiFont } from '../constants/theme';
import {
  frameIndexForKind,
  TILE_DISPLAY_H,
  TILE_DISPLAY_W,
  TILE_FRAME_H,
  TILE_FRAME_W,
  TILE_GAP,
  TILE_SCALE,
  TILES_KEY,
} from '../utils/tiles';

/** 捨て牌: 2行×10列（3段目が切り牌に重ならないように） */
const RIVER_ROW_COUNTS = [10, 10] as const;
const RIVER_MAX_COLS = 10;

export function addTileSprite(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  x: number,
  y: number,
  tile: TileView,
  scale = TILE_SCALE
): Phaser.GameObjects.Image {
  const frame = frameIndexForKind(tile.kind);
  const img = scene.add.image(x, y, TILES_KEY, frame).setScale(scale);
  root.add(img);
  return img;
}

export function drawTileRowCentered(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  tiles: TileView[],
  y: number,
  scale = 0.8
) {
  const tw = Math.round(TILE_FRAME_W * scale);
  const gap = TILE_GAP;
  const totalW = tiles.length * (tw + gap) - gap;
  const startX = (scene.scale.width - totalW) / 2 + tw / 2;
  tiles.forEach((t, i) => {
    addTileSprite(scene, root, startX + i * (tw + gap), y, t, scale);
  });
}

export function drawFaceDownRow(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  count: number,
  y: number,
  scale = 0.72
) {
  const tw = Math.round(TILE_FRAME_W * scale);
  const th = Math.round(TILE_FRAME_H * scale);
  const gap = 3;
  const totalW = count * (tw + gap) - gap;
  const startX = (scene.scale.width - totalW) / 2 + tw / 2;
  for (let i = 0; i < count; i++) {
    const x = startX + i * (tw + gap);
    const back = scene.add.rectangle(x, y, tw - 1, th - 1, 0xf5f0e6, 1);
    back.setStrokeStyle(1, 0xc9c2b4, 1);
    root.add(back);
  }
}

export function riverRowPitch(scale: number): number {
  const th = Math.round(TILE_FRAME_H * scale);
  return th + 5;
}

export function riverBlockHeight(scale: number): number {
  const th = Math.round(TILE_FRAME_H * scale);
  const rows = RIVER_ROW_COUNTS.length;
  return (rows - 1) * riverRowPitch(scale) + th;
}

export function drawRiverGrid(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  tiles: TileView[],
  topCenterY: number,
  opts?: {
    highlightId?: number;
    scale?: number;
    side?: 'self' | 'opponent';
  }
) {
  const scale = opts?.scale ?? 0.7;
  const side = opts?.side ?? 'self';
  const upsideDown = side === 'opponent';
  const tw = Math.round(TILE_FRAME_W * scale);
  const th = Math.round(TILE_FRAME_H * scale);
  const gap = 3;
  const pitch = riverRowPitch(scale);
  const maxCols = RIVER_MAX_COLS;
  const gridW = maxCols * (tw + gap) - gap;
  const left = (scene.scale.width - gridW) / 2;
  let idx = 0;

  const lastRow = RIVER_ROW_COUNTS.length - 1;
  for (let fillRow = 0; fillRow < RIVER_ROW_COUNTS.length; fillRow++) {
    const cols = RIVER_ROW_COUNTS[fillRow];
    const visualRow = side === 'opponent' ? lastRow - fillRow : fillRow;
    const y = topCenterY + visualRow * pitch;

    for (let c = 0; c < cols; c++) {
      if (idx < tiles.length) {
        const t = tiles[idx];
        const colFromLeft = side === 'opponent' ? maxCols - 1 - c : c;
        const x = left + colFromLeft * (tw + gap) + tw / 2;
        const highlight = opts?.highlightId != null && t.id === opts.highlightId;
        if (highlight) {
          const glow = scene.add.rectangle(x, y, tw + 8, th + 8, 0xf5d76e, 0.55);
          glow.setStrokeStyle(2, 0xffe566, 1);
          root.add(glow);
        }
        const img = addTileSprite(scene, root, x, y, t, scale);
        if (upsideDown) img.setAngle(180);
      }
      idx++;
    }
  }
}

export function drawWaitBottomRight(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  waitKinds: string[],
  opts?: { marginRight?: number; y?: number; scale?: number }
) {
  if (!Array.isArray(waitKinds) || waitKinds.length === 0) return;
  const scale = opts?.scale ?? 0.62 * TILE_SIZE_SCALE;
  const tw = Math.round(TILE_FRAME_W * scale);
  const gap = TILE_GAP;
  const marginRight = opts?.marginRight ?? 16;
  const y = opts?.y ?? scene.scale.height - 36;
  const totalW = waitKinds.length * (tw + gap) - gap;
  const tilesRight = scene.scale.width - marginRight;
  const startX = tilesRight - totalW;

  root.add(
    scene.add
      .text(startX - 6, y, '待ち', {
        fontFamily: FONT_UI,
        fontSize: uiFont(11),
        color: UI_COLOR.muted,
      })
      .setOrigin(1, 0.5)
  );

  waitKinds.forEach((kind, i) => {
    const cx = startX + tw / 2 + i * (tw + gap);
    addTileSprite(scene, root, cx, y, { id: -3000 - i, kind }, scale);
  });
}

export { TILE_DISPLAY_H, TILE_DISPLAY_W, TILE_FRAME_H, TILE_FRAME_W, TILE_GAP, TILE_SCALE };
