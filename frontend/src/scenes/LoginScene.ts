import * as Phaser from 'phaser';
import { mountMenuOverlay, setOverlayError, unmountMenuOverlay } from '../utils/menuOverlay';

export class LoginScene extends Phaser.Scene {
  private overlay: HTMLElement | null = null;
  private errorMessage = '';

  constructor() {
    super('LoginScene');
  }

  init(data: { error?: string }) {
    this.errorMessage = data.error || '';
  }

  create() {
    this.events.once('shutdown', this.onShutdown, this);

    const errorHtml = this.errorMessage
      ? `<div class="error-message">${escapeHtml(this.errorMessage)}</div>`
      : '';

    this.overlay = mountMenuOverlay(`
      <div class="glass-container login-shell">
        <div class="login-layout">
          <div class="login-brand">
            <div class="login-title-row">
              <h1>十七歩</h1>
              <div class="login-honor-tiles" aria-hidden="true">
                <span class="mh-tile" data-kind="ton" title="東"></span>
                <span class="mh-tile" data-kind="nan" title="南"></span>
                <span class="mh-tile" data-kind="sha" title="西"></span>
                <span class="mh-tile" data-kind="pei" title="北"></span>
                <span class="mh-tile" data-kind="haku" title="白"></span>
                <span class="mh-tile" data-kind="hatsu" title="發"></span>
                <span class="mh-tile" data-kind="chun" title="中"></span>
              </div>
            </div>
            <p class="subtitle">SeventeenWalk — 2人オンライン対戦</p>
          </div>
          <div class="login-form">
            ${errorHtml}
            <div class="input-group">
              <label class="input-label" for="playerName">プレイヤー名</label>
              <input class="input-field" type="text" id="playerName" maxlength="16" placeholder="例: Alice" />
            </div>
            <div class="login-actions">
              <button type="button" class="action-button" id="startBtn">ロビーへ</button>
              <button type="button" class="action-button secondary" id="creditsBtn">クレジット</button>
            </div>
          </div>
        </div>
      </div>
    `);

    const nameInput = this.overlay.querySelector('#playerName') as HTMLInputElement;
    const startBtn = this.overlay.querySelector('#startBtn') as HTMLButtonElement;
    const creditsBtn = this.overlay.querySelector('#creditsBtn') as HTMLButtonElement;

    nameInput.value = `Player_${Math.floor(Math.random() * 900 + 100)}`;

    startBtn.addEventListener('click', () => {
      const name = nameInput.value.trim();
      if (!name) {
        setOverlayError(this.overlay, 'プレイヤー名を入力してください');
        return;
      }
      this.scene.start('LobbyScene', { name });
    });

    creditsBtn.addEventListener('click', () => {
      this.scene.start('CreditScene');
    });

    nameInput.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter') startBtn.click();
    });
  }

  private onShutdown() {
    unmountMenuOverlay(this.overlay);
    this.overlay = null;
  }
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
