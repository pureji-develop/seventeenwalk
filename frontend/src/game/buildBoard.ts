import * as Phaser from 'phaser';
import type { PublicState, TileView } from '../types/protocol';
import { FONT_UI, PHASER_COLOR, UI_COLOR } from '../constants/theme';
import { sortTilesByKind, TILE_FRAME_W, TILE_GAP } from '../utils/tiles';
import { addTileSprite, TILE_DISPLAY_H, TILE_DISPLAY_W } from './tileSprites';

export type BuildBoardCallbacks = {
  onSelectionChanged: () => void;
  onSubmit: (tileIds: number[]) => void;
  onRiipai: () => void;
  onClear: () => void;
};

/**
 * BUILD phase board. Selection updates rebuild only this layer (not the whole scene root).
 */
export class BuildBoard {
  private layer: Phaser.GameObjects.Container;
  private scene: Phaser.Scene;

  constructor(scene: Phaser.Scene, root: Phaser.GameObjects.Container) {
    this.scene = scene;
    this.layer = scene.add.container(0, 0);
    root.add(this.layer);
  }

  destroy() {
    this.layer.destroy(true);
  }

  clear() {
    this.layer.removeAll(true);
  }

  render(
    s: PublicState,
    selectedBuildIds: number[],
    previewWaits: string[],
    wallFlipById: Map<number, boolean>,
    roundIntroDone: boolean,
    ensureWallFlips: (wall: TileView[]) => void,
    cbs: BuildBoardCallbacks
  ) {
    this.layer.removeAll(true);

    const submitted = s.you.handSubmitted;
    const oppOk = s.opponent.handSubmitted;
    const interactive = roundIntroDone && !submitted;
    const selectedSet = new Set(selectedBuildIds);
    const wallRaw = s.you.wall ?? [];
    const wallById = new Map(wallRaw.map((t) => [t.id, t]));
    const wallSorted = s.wallSorted === true;
    const wall = wallSorted ? sortTilesByKind(wallRaw) : wallRaw;
    if (!wallSorted) {
      ensureWallFlips(wall);
    } else {
      wallFlipById.clear();
    }

    let statusMsg: string;
    if (submitted && oppOk) {
      statusMsg = '両者 OK — 捨て合いへ…';
    } else if (submitted) {
      statusMsg = 'OK 済み — 相手の OK を待っています';
    } else if (oppOk) {
      statusMsg = `相手は OK 済み — 13枚選んで OK（${selectedBuildIds.length}/13）`;
    } else {
      statusMsg = `13枚選んで OK（${selectedBuildIds.length}/13）※両者 OK で捨て牌開始`;
    }

    const statusY = 118;
    const startY = 168;
    this.layer.add(
      this.scene.add
        .text(this.scene.scale.width / 2, statusY, statusMsg, {
          fontFamily: FONT_UI,
          fontSize: '16px',
          color: UI_COLOR.ivory,
        })
        .setOrigin(0.5)
    );

    const cols = 17;
    const startX =
      (this.scene.scale.width - cols * (TILE_DISPLAY_W + TILE_GAP)) / 2 + TILE_DISPLAY_W / 2;

    wall.forEach((t, i) => {
      const col = i % cols;
      const row = Math.floor(i / cols);
      const x = startX + col * (TILE_DISPLAY_W + TILE_GAP);
      const y = startY + row * (TILE_DISPLAY_H + TILE_GAP + 8);

      if (selectedSet.has(t.id)) {
        const slot = this.scene.add.rectangle(
          x,
          y,
          TILE_DISPLAY_W - 2,
          TILE_DISPLAY_H - 2,
          PHASER_COLOR.panel,
          0.35
        );
        slot.setStrokeStyle(1, 0x8a9aab, 0.25);
        this.layer.add(slot);
        return;
      }

      const upsideDown = !wallSorted && wallFlipById.get(t.id) === true;
      const img = addTileSprite(this.scene, this.layer, x, y, t);
      if (upsideDown) img.setAngle(180);
      if (interactive) {
        img.setInteractive({ useHandCursor: true });
        img.on('pointerover', () => img.setY(y - 6));
        img.on('pointerout', () => img.setY(y));
        img.on('pointerdown', () => {
          if (selectedBuildIds.length >= 13) return;
          selectedBuildIds.push(t.id);
          cbs.onSelectionChanged();
        });
      } else if (submitted) {
        img.setAlpha(0.4);
      }
    });

    const handY = this.scene.scale.height - 100;
    const waitKinds = resolveWaitKinds(s, submitted, selectedBuildIds, previewWaits);
    const buttonRowY = this.scene.scale.height - 36;
    // OK ボタン中心 + 半幅 + 余白（理牌/クリア/OK の右側）
    const waitsStartX = this.scene.scale.width / 2 + 140 + 58;

    const displayHand: TileView[] = submitted
      ? (s.you.hand ?? [])
      : selectedBuildIds.map((id) => wallById.get(id)).filter((t): t is TileView => Boolean(t));

    const handStartX =
      (this.scene.scale.width - Math.max(displayHand.length, 1) * (TILE_DISPLAY_W + TILE_GAP) + TILE_GAP) /
        2 +
      TILE_DISPLAY_W / 2;

    displayHand.forEach((t, i) => {
      const x = handStartX + i * (TILE_DISPLAY_W + TILE_GAP);
      const img = addTileSprite(this.scene, this.layer, x, handY, t);
      if (interactive) {
        img.setInteractive({ useHandCursor: true });
        img.on('pointerdown', () => {
          const idx = selectedBuildIds.indexOf(t.id);
          if (idx >= 0) selectedBuildIds.splice(idx, 1);
          cbs.onSelectionChanged();
        });
      }
    });

    if (displayHand.length === 0) {
      this.layer.add(
        this.scene.add
          .text(this.scene.scale.width / 2, handY, '（ここへ移動）', {
            fontFamily: FONT_UI,
            fontSize: '13px',
            color: UI_COLOR.muted,
          })
          .setOrigin(0.5)
      );
    }

    if (interactive) {
      const hasSelection = selectedBuildIds.length > 0;
      addActionButton(
        this.scene,
        this.layer,
        this.scene.scale.width / 2 - 140,
        this.scene.scale.height - 36,
        '理牌',
        () => cbs.onRiipai(),
        hasSelection
      );
      addActionButton(
        this.scene,
        this.layer,
        this.scene.scale.width / 2,
        this.scene.scale.height - 36,
        'クリア',
        () => cbs.onClear(),
        hasSelection
      );
      const canOk = selectedBuildIds.length === 13;
      addActionButton(
        this.scene,
        this.layer,
        this.scene.scale.width / 2 + 140,
        this.scene.scale.height - 36,
        'OK',
        () => {
          if (!canOk) return;
          cbs.onSubmit([...selectedBuildIds]);
        },
        canOk
      );
      drawWaitBesideButtons(this.scene, this.layer, waitKinds, waitsStartX, buttonRowY);
    } else if (submitted) {
      this.layer.add(
        this.scene.add
          .text(
            this.scene.scale.width / 2,
            this.scene.scale.height - 36,
            oppOk ? '捨て合い開始…' : '相手の OK 待ち',
            {
              fontFamily: FONT_UI,
              fontSize: '15px',
              color: UI_COLOR.muted,
            }
          )
          .setOrigin(0.5)
      );
      drawWaitBesideButtons(this.scene, this.layer, waitKinds, waitsStartX, buttonRowY);
    }
  }
}

function resolveWaitKinds(
  s: PublicState,
  submitted: boolean,
  selectedBuildIds: number[],
  previewWaits: string[]
): string[] {
  if (submitted) {
    return Array.isArray(s.you.waits) ? s.you.waits : [];
  }
  if (selectedBuildIds.length !== 13) return [];
  return Array.isArray(previewWaits) ? previewWaits : [];
}

function drawWaitBesideButtons(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  waitKinds: string[],
  startX: number,
  y: number,
  scale = 0.62
) {
  if (!Array.isArray(waitKinds) || waitKinds.length === 0) return;
  const tw = Math.round(TILE_FRAME_W * scale);
  const gap = TILE_GAP;
  root.add(
    scene.add
      .text(startX - 4, y, '待ち', {
        fontFamily: FONT_UI,
        fontSize: '11px',
        color: UI_COLOR.muted,
      })
      .setOrigin(1, 0.5)
  );
  waitKinds.forEach((kind, i) => {
    const tile: TileView = { id: -3000 - i, kind };
    const cx = startX + tw / 2 + i * (tw + gap);
    addTileSprite(scene, root, cx, y, tile, scale);
  });
}

function addActionButton(
  scene: Phaser.Scene,
  root: Phaser.GameObjects.Container,
  x: number,
  y: number,
  label: string,
  onClick: () => void,
  enabled: boolean
) {
  const btn = scene.add
    .text(x, y, label, {
      fontFamily: FONT_UI,
      fontSize: '16px',
      fontStyle: 'bold',
      color: enabled ? UI_COLOR.ivory : UI_COLOR.muted,
      backgroundColor: enabled ? UI_COLOR.accent : '#2a4a3c',
      padding: { x: 22, y: 12 },
    })
    .setOrigin(0.5);
  root.add(btn);
  if (enabled) {
    btn.setInteractive({ useHandCursor: true });
    btn.on('pointerover', () => btn.setStyle({ backgroundColor: '#f07060' }));
    btn.on('pointerout', () => btn.setStyle({ backgroundColor: UI_COLOR.accent }));
    btn.on('pointerdown', onClick);
  }
  return btn;
}
