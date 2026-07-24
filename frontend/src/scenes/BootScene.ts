import * as Phaser from 'phaser';
import { hideBootLoader } from '../utils/bootLoader';
import { loadTileMeta, TILE_FRAME_H, TILE_FRAME_W, TILES_KEY, TILES_PATH } from '../utils/tiles';

export class BootScene extends Phaser.Scene {
  constructor() {
    super('BootScene');
  }

  preload() {
    this.load.on('loaderror', (file: Phaser.Loader.File) => {
      console.error('[BootScene] failed to load', file?.key, file?.url);
    });
    this.load.spritesheet(TILES_KEY, TILES_PATH, {
      frameWidth: TILE_FRAME_W,
      frameHeight: TILE_FRAME_H,
    });
  }

  async create() {
    try {
      await loadTileMeta();
    } catch (err) {
      console.error('tiles.json load failed', err);
    }

    const body = document.body;
    if (!document.querySelector('.bg-blob.blob-ink')) {
      const blobInk = document.createElement('div');
      blobInk.className = 'bg-blob blob-ink';
      body.appendChild(blobInk);
      const blobLacquer = document.createElement('div');
      blobLacquer.className = 'bg-blob blob-lacquer';
      body.appendChild(blobLacquer);
    }

    hideBootLoader();
    this.scene.start('LoginScene');
  }
}
