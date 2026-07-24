import * as Phaser from 'phaser';
import './style.css';
import { GAME_HEIGHT, GAME_WIDTH, UI_COLOR } from './constants/theme';
import { BootScene } from './scenes/BootScene';
import { LoginScene } from './scenes/LoginScene';
import { LobbyScene } from './scenes/LobbyScene';
import { GameScene } from './scenes/GameScene';
import { CreditScene } from './scenes/CreditScene';
import { hideBootLoader } from './utils/bootLoader';

const config: Phaser.Types.Core.GameConfig = {
  type: Phaser.AUTO,
  width: GAME_WIDTH,
  height: GAME_HEIGHT,
  parent: 'app',
  backgroundColor: UI_COLOR.table,
  banner: false,
  dom: {
    createContainer: true,
  },
  scale: {
    mode: Phaser.Scale.FIT,
    autoCenter: Phaser.Scale.CENTER_BOTH,
  },
  scene: [BootScene, LoginScene, LobbyScene, GameScene, CreditScene],
};

const game = new Phaser.Game(config);

game.events.once('ready', () => {
  if (game.canvas) {
    game.canvas.style.zIndex = '1';
  }
  window.setTimeout(() => hideBootLoader(), 8000);
});
