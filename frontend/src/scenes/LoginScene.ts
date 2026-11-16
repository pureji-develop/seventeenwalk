import * as Phaser from 'phaser';
import {
  createRoom,
  DEFAULT_ROOM_ID,
  fetchOpenRooms,
  fetchRoomInfo,
  isDevClient,
  type PublicRoomInfo,
} from '../constants/network';
import { mountMenuOverlay, setOverlayError, unmountMenuOverlay } from '../utils/menuOverlay';

export class LoginScene extends Phaser.Scene {
  private overlay: HTMLElement | null = null;
  private errorMessage = '';
  private roomsTimer: ReturnType<typeof setInterval> | null = null;

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
            <div class="input-group">
              <label class="input-label" for="roomId">ルームID（参加する場合）</label>
              <input class="input-field" type="text" id="roomId" placeholder="${DEFAULT_ROOM_ID}" />
            </div>
            <div class="login-actions">
              <button type="button" class="action-button" id="createBtn">ルームを作成して入室</button>
              <button type="button" class="action-button secondary" id="joinBtn">ルームに参加</button>
              <button type="button" class="action-button secondary" id="creditsBtn">クレジット</button>
            </div>
            <div class="login-rooms">
              <div class="login-rooms-header">
                <span class="input-label">対戦待ちルーム</span>
                <button type="button" class="lobby-chip" id="refreshRoomsBtn">更新</button>
              </div>
              <div class="login-rooms-list" id="roomsList">
                <div class="login-rooms-empty">読み込み中...</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    `);

    const nameInput = this.overlay.querySelector('#playerName') as HTMLInputElement;
    const roomInput = this.overlay.querySelector('#roomId') as HTMLInputElement;
    const createBtn = this.overlay.querySelector('#createBtn') as HTMLButtonElement;
    const joinBtn = this.overlay.querySelector('#joinBtn') as HTMLButtonElement;
    const creditsBtn = this.overlay.querySelector('#creditsBtn') as HTMLButtonElement;
    const refreshBtn = this.overlay.querySelector('#refreshRoomsBtn') as HTMLButtonElement;

    nameInput.value = `Player_${Math.floor(Math.random() * 900 + 100)}`;
    if (isDevClient()) {
      roomInput.value = DEFAULT_ROOM_ID;
    }

    const enterLobby = (name: string, roomId: string) => {
      this.scene.start('LobbyScene', { name, roomId });
    };

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
        enterLobby(name, roomId);
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
        enterLobby(name, roomId);
      } catch (err) {
        setOverlayError(this.overlay, err instanceof Error ? err.message : String(err));
        joinBtn.disabled = false;
      }
    });

    creditsBtn.addEventListener('click', () => {
      this.scene.start('CreditScene');
    });

    refreshBtn.addEventListener('click', () => {
      void this.refreshRooms();
    });

    this.overlay.querySelector('#roomsList')?.addEventListener('click', (ev) => {
      const btn = (ev.target as HTMLElement).closest('button[data-challenge]') as HTMLButtonElement | null;
      if (!btn) return;
      const roomId = btn.dataset.challenge;
      if (!roomId) return;
      const name = nameInput.value.trim();
      if (!name) {
        setOverlayError(this.overlay, 'プレイヤー名を入力してください');
        return;
      }
      roomInput.value = roomId;
      setOverlayError(this.overlay, '');
      enterLobby(name, roomId);
    });

    void this.refreshRooms();
    this.roomsTimer = setInterval(() => {
      void this.refreshRooms();
    }, 8000);
  }

  private async refreshRooms() {
    const box = this.overlay?.querySelector('#roomsList');
    if (!box) return;
    try {
      const rooms = await fetchOpenRooms();
      this.renderRooms(rooms);
    } catch (err) {
      box.innerHTML = `<div class="login-rooms-empty">${escapeHtml(
        err instanceof Error ? err.message : '一覧を取得できませんでした'
      )}</div>`;
    }
  }

  private renderRooms(rooms: PublicRoomInfo[]) {
    const box = this.overlay?.querySelector('#roomsList');
    if (!box) return;
    const open = rooms.filter((r) => r.open);
    if (open.length === 0) {
      box.innerHTML = '<div class="login-rooms-empty">いま挑戦できるルームはありません</div>';
      return;
    }
    box.innerHTML = open
      .map((r) => {
        const rounds = r.totalRounds ? `${r.totalRounds}局` : '';
        const meta = [rounds, r.hostName ? `ホスト: ${escapeHtml(r.hostName)}` : '']
          .filter(Boolean)
          .join(' · ');
        return `
          <div class="login-room-row">
            <div class="login-room-meta">
              <div class="login-room-id">${escapeHtml(r.roomId)}</div>
              <div class="login-room-sub">${meta}</div>
            </div>
            <button type="button" class="action-button" data-challenge="${escapeHtml(r.roomId)}">挑戦</button>
          </div>`;
      })
      .join('');
  }

  private onShutdown() {
    if (this.roomsTimer != null) {
      clearInterval(this.roomsTimer);
      this.roomsTimer = null;
    }
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
