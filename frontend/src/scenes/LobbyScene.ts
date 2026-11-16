import * as Phaser from 'phaser';
import { NetworkManager } from '../network/NetworkManager';
import type { PublicState } from '../types/protocol';
import {
  mountMenuOverlay,
  setOverlayError,
  showDomToast,
  unmountMenuOverlay,
} from '../utils/menuOverlay';

type TimeoutPreset = '120' | '180' | '300' | 'inf';
type RoundsPreset = 4 | 6 | 8 | 10;

const TIMEOUT_OPTIONS: { preset: TimeoutPreset; label: string }[] = [
  { preset: '120', label: '2分' },
  { preset: '180', label: '3分' },
  { preset: '300', label: '5分' },
  { preset: 'inf', label: '無限（両者承認）' },
];

const ROUNDS_OPTIONS: { rounds: RoundsPreset; label: string }[] = [
  { rounds: 4, label: '4局' },
  { rounds: 6, label: '6局' },
  { rounds: 8, label: '8局' },
  { rounds: 10, label: '10局' },
];

function presetFromState(state: PublicState): TimeoutPreset {
  if (state.buildInfinite || state.buildTimeoutSec === 0) return 'inf';
  const sec = state.buildTimeoutSec;
  if (sec === 120 || sec === 180 || sec === 300) return String(sec) as TimeoutPreset;
  return '180';
}

function roundsFromState(state: PublicState): RoundsPreset {
  const n = state.totalRounds;
  if (n === 4 || n === 6 || n === 8 || n === 10) return n;
  return 4;
}

function secFromPreset(preset: TimeoutPreset): number {
  return preset === 'inf' ? 0 : Number(preset);
}

export class LobbyScene extends Phaser.Scene {
  private name = '';
  private roomId = '';
  private network: NetworkManager | null = null;
  private overlay: HTMLElement | null = null;
  private handedOffSocket = false;
  private lastState: PublicState | null = null;
  /** Local draft; synced from server state, sent via set_options. */
  private draftPreset: TimeoutPreset = '180';
  private draftRounds: RoundsPreset = 4;
  private draftWallSorted = false;
  private syncingOptions = false;
  private challengeTick: ReturnType<typeof setInterval> | null = null;
  private challengeDenied = false;

  constructor() {
    super('LobbyScene');
  }

  init(data: { name: string; roomId: string }) {
    this.name = data.name;
    this.roomId = data.roomId;
    this.handedOffSocket = false;
    this.lastState = null;
    this.draftPreset = '180';
    this.draftRounds = 4;
    this.draftWallSorted = false;
    this.challengeDenied = false;
  }

  create() {
    this.events.once('shutdown', this.onShutdown, this);

    this.overlay = mountMenuOverlay(`
      <div class="glass-container lobby-shell">
        <div class="lobby-layout">
          <div class="lobby-main">
            <div class="lobby-room-info" id="roomLabel">ROOM: ${this.roomId}</div>
            <p class="lobby-hint">このルームIDを相手に共有してください（2人対戦）</p>
            <div class="input-label" style="text-align:left;margin-bottom:6px;">対戦相手</div>
            <div class="lobby-players-box" id="playerList">
              <div style="color:#8a9aab;text-align:center;font-size:0.9rem;">接続中...</div>
            </div>
            <div class="lobby-challenge-wait hidden" id="challengeWait">
              ホストの承認を待っています…
              <span id="challengeWaitRemain"></span>
            </div>
            <div class="lobby-actions-row lobby-actions-primary">
              <button type="button" class="action-button" id="readyBtn">準備OK</button>
              <button type="button" class="action-button secondary" id="leaveBtn">退室</button>
            </div>
            <button type="button" class="action-button secondary" id="copyBtn">ルームIDをコピー</button>
          </div>
          <div class="lobby-side">
            <div class="lobby-options">
              <div class="input-label" style="text-align:left;margin-bottom:8px;">局数</div>
              <div class="lobby-option-row" id="roundsOpts" role="group" aria-label="局数"></div>

              <div class="input-label" style="text-align:left;margin:14px 0 8px;">制限時間</div>
              <div class="lobby-option-row" id="timeoutOpts" role="group" aria-label="制限時間"></div>

              <div class="input-label" style="text-align:left;margin:14px 0 8px;">山札理牌</div>
              <div class="lobby-option-row" id="wallSortOpts" role="group" aria-label="山札理牌"></div>
              <p class="lobby-option-hint">変更すると両者の準備が解除されます</p>
            </div>
          </div>
        </div>
        <div class="challenge-modal hidden" id="challengeModal" role="dialog" aria-modal="true">
          <div class="challenge-modal-card">
            <p class="challenge-modal-text" id="challengeModalText"></p>
            <p class="challenge-modal-timer" id="challengeModalTimer"></p>
            <div class="lobby-actions-row">
              <button type="button" class="action-button" id="challengeAcceptBtn">始める</button>
              <button type="button" class="action-button secondary" id="challengeDeclineBtn">断る</button>
            </div>
          </div>
        </div>
      </div>
    `);

    this.renderOptionButtons();
    this.bindOptionClicks();

    const readyBtn = this.overlay.querySelector('#readyBtn') as HTMLButtonElement;
    const leaveBtn = this.overlay.querySelector('#leaveBtn') as HTMLButtonElement;
    const copyBtn = this.overlay.querySelector('#copyBtn') as HTMLButtonElement;

    readyBtn.addEventListener('click', () => {
      if (!this.network?.isOpen) return;
      if (this.lastState?.challenge) {
        setOverlayError(this.overlay, '挑戦への応答後に準備できます');
        return;
      }
      const next = !(this.lastState?.you.ready ?? false);
      this.network.send('ready', { ready: next });
    });

    leaveBtn.addEventListener('click', () => {
      this.network?.send('leave', {});
      this.network?.close();
      this.scene.start('LoginScene');
    });

    copyBtn.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(this.roomId);
        showDomToast('ルームIDをコピーしました');
      } catch {
        showDomToast(this.roomId, { title: 'ルームID（手動コピー）' });
      }
    });

    const acceptBtn = this.overlay.querySelector('#challengeAcceptBtn') as HTMLButtonElement;
    const declineBtn = this.overlay.querySelector('#challengeDeclineBtn') as HTMLButtonElement;
    acceptBtn.addEventListener('click', () => {
      this.network?.send('respond_challenge', { accept: true });
    });
    declineBtn.addEventListener('click', () => {
      this.network?.send('respond_challenge', { accept: false });
    });

    this.network = new NetworkManager({ roomId: this.roomId, name: this.name });
    this.network.on('state', (state) => this.onState(state));
    this.network.on('error', (msg) => {
      if (
        msg.includes('挑戦') ||
        msg.includes('タイムアウト') ||
        msg.includes('拒否') ||
        msg.includes('ROOM_BUSY') ||
        msg.includes('ROOM_FULL')
      ) {
        this.challengeDenied = true;
        this.network?.close();
        this.scene.start('LoginScene', { error: msg });
        return;
      }
      setOverlayError(this.overlay, msg);
      // 失敗したらサーバ状態に戻す
      if (this.lastState) {
        this.draftPreset = presetFromState(this.lastState);
        this.draftRounds = roundsFromState(this.lastState);
        this.draftWallSorted = this.lastState.wallSorted ?? false;
        this.syncOptionChipActive();
      }
    });
    this.network.on('disconnect', (msg) => {
      if (!this.handedOffSocket && !this.challengeDenied) {
        this.scene.start('LoginScene', { error: msg });
      }
    });
    this.network.on('reconnecting', (msg) => {
      if (!this.handedOffSocket) {
        showDomToast(msg);
      }
    });
    this.network.connect();
  }

  private renderOptionButtons() {
    const roundsBox = this.overlay?.querySelector('#roundsOpts');
    const timeoutBox = this.overlay?.querySelector('#timeoutOpts');
    const wallBox = this.overlay?.querySelector('#wallSortOpts');
    if (!roundsBox || !timeoutBox || !wallBox) return;

    roundsBox.innerHTML = ROUNDS_OPTIONS.map(
      (o) =>
        `<button type="button" class="lobby-chip" data-rounds="${o.rounds}">${o.label}</button>`
    ).join('');

    timeoutBox.innerHTML = TIMEOUT_OPTIONS.map(
      (o) =>
        `<button type="button" class="lobby-chip" data-preset="${o.preset}">${o.label}</button>`
    ).join('');

    wallBox.innerHTML = `
      <button type="button" class="lobby-chip" data-wall="1">あり</button>
      <button type="button" class="lobby-chip" data-wall="0">なし</button>
    `;

    this.syncOptionChipActive();
  }

  private bindOptionClicks() {
    const roundsBox = this.overlay?.querySelector('#roundsOpts');
    const timeoutBox = this.overlay?.querySelector('#timeoutOpts');
    const wallBox = this.overlay?.querySelector('#wallSortOpts');
    if (!roundsBox || !timeoutBox || !wallBox) return;

    roundsBox.addEventListener('click', (ev) => {
      const btn = (ev.target as HTMLElement).closest('button[data-rounds]') as HTMLButtonElement | null;
      if (!btn || this.syncingOptions) return;
      const n = Number(btn.dataset.rounds);
      if (n !== 4 && n !== 6 && n !== 8 && n !== 10) return;
      this.draftRounds = n;
      this.syncOptionChipActive();
      this.sendOptions();
    });

    timeoutBox.addEventListener('click', (ev) => {
      const btn = (ev.target as HTMLElement).closest('button[data-preset]') as HTMLButtonElement | null;
      if (!btn || this.syncingOptions) return;
      const preset = btn.dataset.preset as TimeoutPreset | undefined;
      if (!preset || !TIMEOUT_OPTIONS.some((o) => o.preset === preset)) return;
      this.draftPreset = preset;
      this.syncOptionChipActive();
      this.sendOptions();
    });

    wallBox.addEventListener('click', (ev) => {
      const btn = (ev.target as HTMLElement).closest('button[data-wall]') as HTMLButtonElement | null;
      if (!btn || this.syncingOptions) return;
      this.draftWallSorted = btn.dataset.wall === '1';
      this.syncOptionChipActive();
      this.sendOptions();
    });
  }

  private syncOptionChipActive() {
    const roundsBox = this.overlay?.querySelector('#roundsOpts');
    const timeoutBox = this.overlay?.querySelector('#timeoutOpts');
    const wallBox = this.overlay?.querySelector('#wallSortOpts');
    roundsBox?.querySelectorAll('button[data-rounds]').forEach((el) => {
      const btn = el as HTMLButtonElement;
      btn.classList.toggle('active', Number(btn.dataset.rounds) === this.draftRounds);
    });
    timeoutBox?.querySelectorAll('button[data-preset]').forEach((el) => {
      const btn = el as HTMLButtonElement;
      btn.classList.toggle('active', btn.dataset.preset === this.draftPreset);
    });
    wallBox?.querySelectorAll('button[data-wall]').forEach((el) => {
      const btn = el as HTMLButtonElement;
      const want = btn.dataset.wall === '1';
      btn.classList.toggle('active', want === this.draftWallSorted);
    });
  }

  private sendOptions() {
    if (!this.network?.isOpen) return;
    setOverlayError(this.overlay, '');
    this.network.send('set_options', {
      timeoutPreset: this.draftPreset,
      buildTimeoutSec: secFromPreset(this.draftPreset),
      totalRounds: this.draftRounds,
      wallSorted: this.draftWallSorted,
    });
  }

  private onState(state: PublicState) {
    this.lastState = state;
    this.registry.set('gameState', state);
    this.renderPlayers(state);
    this.renderChallenge(state);
    setOverlayError(this.overlay, '');

    const readyBtn = this.overlay?.querySelector('#readyBtn') as HTMLButtonElement | null;
    const pending = Boolean(state.challenge);
    if (readyBtn) {
      readyBtn.textContent = state.you.ready ? '準備解除' : '準備OK';
      readyBtn.classList.toggle('ready-on', state.you.ready);
      readyBtn.disabled = pending;
    }

    // Sync options from server (either player can change; both see the same).
    const preset = presetFromState(state);
    const rounds = roundsFromState(state);
    const wall = state.wallSorted ?? this.draftWallSorted;
    if (
      preset !== this.draftPreset ||
      rounds !== this.draftRounds ||
      wall !== this.draftWallSorted
    ) {
      this.syncingOptions = true;
      this.draftPreset = preset;
      this.draftRounds = rounds;
      this.draftWallSorted = wall;
      this.syncOptionChipActive();
      this.syncingOptions = false;
    } else {
      this.syncOptionChipActive();
    }

    if (state.phase && state.phase !== 'LOBBY') {
      this.transitionToGame();
    }
  }

  private renderChallenge(state: PublicState) {
    const modal = this.overlay?.querySelector('#challengeModal') as HTMLElement | null;
    const wait = this.overlay?.querySelector('#challengeWait') as HTMLElement | null;
    const text = this.overlay?.querySelector('#challengeModalText') as HTMLElement | null;
    const ch = state.challenge;
    if (!ch) {
      modal?.classList.add('hidden');
      wait?.classList.add('hidden');
      this.stopChallengeTick();
      return;
    }
    if (ch.role === 'host') {
      wait?.classList.add('hidden');
      modal?.classList.remove('hidden');
      if (text) {
        text.textContent = `${ch.fromName}さんから挑戦されました、始めますか？`;
      }
    } else {
      modal?.classList.add('hidden');
      wait?.classList.remove('hidden');
    }
    this.startChallengeTick(ch.expiresAtMs);
  }

  private startChallengeTick(expiresAtMs: number) {
    this.stopChallengeTick();
    const update = () => {
      const remainSec = Math.max(0, Math.ceil((expiresAtMs - Date.now()) / 1000));
      const label = `残り ${remainSec} 秒`;
      const modalTimer = this.overlay?.querySelector('#challengeModalTimer');
      const waitRemain = this.overlay?.querySelector('#challengeWaitRemain');
      if (modalTimer) modalTimer.textContent = label;
      if (waitRemain) waitRemain.textContent = `（${label}）`;
    };
    update();
    this.challengeTick = setInterval(update, 250);
  }

  private stopChallengeTick() {
    if (this.challengeTick != null) {
      clearInterval(this.challengeTick);
      this.challengeTick = null;
    }
  }

  private renderPlayers(state: PublicState) {
    const box = this.overlay?.querySelector('#playerList');
    if (!box) return;

    const youReady = state.you.ready
      ? '<span class="badge ready">READY</span>'
      : '<span class="badge">WAITING</span>';

    const oppPresent = Boolean(state.opponent?.name);
    let oppHtml: string;
    if (!oppPresent) {
      oppHtml = `
        <div class="lobby-player-row">
          <span>空き席</span>
          <span class="badge empty">WAITING FOR OPPONENT</span>
        </div>`;
    } else {
      const reconnecting = state.opponent.connected === false;
      const oppReady = reconnecting
        ? '<span class="badge empty">再接続待ち</span>'
        : state.opponent.ready
          ? '<span class="badge ready">READY</span>'
          : '<span class="badge">WAITING</span>';
      oppHtml = `
        <div class="lobby-player-row">
          <span>${escapeHtml(state.opponent.name)} (Seat ${state.opponent.seat})</span>
          ${oppReady}
        </div>`;
    }

    box.innerHTML = `
      <div class="lobby-player-row">
        <span>${escapeHtml(state.you.name)} (あなた / Seat ${state.you.seat})</span>
        ${youReady}
      </div>
      ${oppHtml}
    `;
  }

  private transitionToGame() {
    if (this.handedOffSocket || !this.network) return;
    this.handedOffSocket = true;
    const socket = this.network.detach();
    this.scene.start('GameScene', {
      socket,
      name: this.name,
      roomId: this.roomId,
      playerId: this.network.getPlayerId(),
    });
  }

  private onShutdown() {
    this.stopChallengeTick();
    unmountMenuOverlay(this.overlay);
    this.overlay = null;
    if (!this.handedOffSocket) {
      this.network?.close();
    }
    this.network = null;
  }
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
