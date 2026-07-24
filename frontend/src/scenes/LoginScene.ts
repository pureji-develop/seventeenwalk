import * as Phaser from 'phaser';
import { createRoom, DEFAULT_ROOM_ID, fetchRoomInfo, isDevClient } from '../constants/network';
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
      ? `<div class="error-message">${this.errorMessage}</div>`
      : '';

    this.overlay = mountMenuOverlay(`
      <div class="glass-container">
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
        ${errorHtml}

        <div class="input-group">
          <label class="input-label" for="playerName">プレイヤー名</label>
          <input class="input-field" type="text" id="playerName" maxlength="16" placeholder="例: Alice" />
        </div>

        <div class="input-group">
          <label class="input-label" for="roomId">ルームID（参加する場合）</label>
          <input class="input-field" type="text" id="roomId" placeholder="${DEFAULT_ROOM_ID}" />
        </div>

        <button type="button" class="action-button" id="createBtn">ルームを作成して入室</button>
        <button type="button" class="action-button secondary" id="joinBtn">ルームに参加</button>
        <button type="button" class="action-button secondary" id="creditsBtn">クレジット</button>
      </div>
    `);

    const nameInput = this.overlay.querySelector('#playerName') as HTMLInputElement;
    const roomInput = this.overlay.querySelector('#roomId') as HTMLInputElement;
    const createBtn = this.overlay.querySelector('#createBtn') as HTMLButtonElement;
    const joinBtn = this.overlay.querySelector('#joinBtn') as HTMLButtonElement;
    const creditsBtn = this.overlay.querySelector('#creditsBtn') as HTMLButtonElement;

    nameInput.value = `Player_${Math.floor(Math.random() * 900 + 100)}`;
    if (isDevClient()) {
      roomInput.value = DEFAULT_ROOM_ID;
    }

    createBtn.addEventListener('click', async () => {
      const name = nameInput.value.trim();
      if (!name) {
        setOverlayError(this.overlay, 'プレイヤー名を入力してください');
        return;
      }
      setOverlayError(this.overlay, '');
      createBtn.disabled = true;
      try {
        const roomId = await createRoom();
        this.scene.start('LobbyScene', { name, roomId });
      } catch (err) {
        setOverlayError(this.overlay, err instanceof Error ? err.message : String(err));
        createBtn.disabled = false;
      }
    });

    joinBtn.addEventListener('click', async () => {
      const name = nameInput.value.trim();
      const roomId = roomInput.value.trim() || (isDevClient() ? DEFAULT_ROOM_ID : '');
      if (!name || !roomId) {
        setOverlayError(this.overlay, 'プレイヤー名とルームIDを入力してください');
        return;
      }
      setOverlayError(this.overlay, '');
      joinBtn.disabled = true;
      try {
        await fetchRoomInfo(roomId);
        this.scene.start('LobbyScene', { name, roomId });
      } catch (err) {
        setOverlayError(this.overlay, err instanceof Error ? err.message : String(err));
        joinBtn.disabled = false;
      }
    });

    creditsBtn.addEventListener('click', () => {
      this.scene.start('CreditScene');
    });
  }

  private onShutdown() {
    unmountMenuOverlay(this.overlay);
    this.overlay = null;
  }
}
