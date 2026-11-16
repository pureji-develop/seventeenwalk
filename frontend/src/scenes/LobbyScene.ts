import * as Phaser from 'phaser';
import { createRoom, fetchOpenRooms, type PublicRoomInfo } from '../constants/network';
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
  private draftPreset: TimeoutPreset = '180';
  private draftRounds: RoundsPreset = 4;
  private draftWallSorted = false;
  private syncingOptions = false;
  private challengeTick: ReturnType<typeof setInterval> | null = null;
  private roomsTimer: ReturnType<typeof setInterval> | null = null;
  private fatalLeave = false;
  private openRooms: PublicRoomInfo[] = [];
  /** After host create, push draft rules once via set_options. */
  private pendingApplyOptions = false;
  /** Host clicked 始める — used to recover if BUILD state is missed. */
  private acceptSent = false;
  private startMatchKickSent = false;
  private startWatchTimer: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    super('LobbyScene');
  }

  init(data: { name: string }) {
    this.name = data.name;
    this.roomId = '';
    this.handedOffSocket = false;
    this.lastState = null;
    this.draftPreset = '180';
    this.draftRounds = 4;
    this.draftWallSorted = false;
    this.fatalLeave = false;
    this.pendingApplyOptions = false;
    this.acceptSent = false;
    this.startMatchKickSent = false;
    this.network = null;
  }

  create() {
    this.events.once('shutdown', this.onShutdown, this);

    this.overlay = mountMenuOverlay(`
      <div class="glass-container lobby-shell">
        <div class="lobby-layout lobby-layout--single">
          <div class="lobby-main">
            <div class="lobby-greeting">こんにちは、<strong id="lobbyName"></strong> さん</div>
            <p class="lobby-hint" id="lobbyHint">部屋を建てて待つか、対戦待ちルームに挑戦してください</p>

            <div id="idlePanel">
              <button type="button" class="action-button" id="hostBtn">部屋を建てて待つ</button>
            </div>

            <div id="matchPanel" class="hidden">
              <div class="input-label" style="text-align:left;margin-bottom:6px;">対戦相手</div>
              <div class="lobby-players-box" id="playerList">
                <div style="color:#8a9aab;text-align:center;font-size:0.9rem;">接続中...</div>
              </div>
              <div class="lobby-challenge-wait hidden" id="challengeWait">
                ホストの承認を待っています…
                <span id="challengeWaitRemain"></span>
              </div>
              <button type="button" class="action-button secondary" id="leaveBtn">退室</button>
            </div>

            <div class="login-rooms lobby-rooms-block">
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
      <div class="challenge-modal host-rules-modal hidden" id="hostRulesModal" role="dialog" aria-modal="true">
        <div class="challenge-modal-card host-rules-card">
          <div class="lobby-options host-rules-options">
            <div class="input-label">局数</div>
            <div class="lobby-option-row" id="roundsOpts" role="group" aria-label="局数"></div>

            <div class="input-label">制限時間</div>
            <div class="lobby-option-row" id="timeoutOpts" role="group" aria-label="制限時間"></div>

            <div class="input-label">山札理牌</div>
            <div class="lobby-option-row" id="wallSortOpts" role="group" aria-label="山札理牌"></div>
          </div>
          <div class="lobby-actions-row host-rules-actions">
            <button type="button" class="action-button" id="hostRulesOkBtn">ルーム作成</button>
            <button type="button" class="action-button secondary" id="hostRulesCancelBtn">キャンセル</button>
          </div>
        </div>
      </div>
      <div class="challenge-modal host-rules-modal hidden" id="challengeModal" role="dialog" aria-modal="true">
        <div class="challenge-modal-card">
          <p class="challenge-modal-text" id="challengeModalText"></p>
          <p class="challenge-modal-timer" id="challengeModalTimer"></p>
          <div class="lobby-actions-row">
            <button type="button" class="action-button" id="challengeAcceptBtn">始める</button>
            <button type="button" class="action-button secondary" id="challengeDeclineBtn">断る</button>
          </div>
        </div>
      </div>
      <div class="challenge-modal host-rules-modal hidden" id="rulesModal" role="dialog" aria-modal="true">
        <div class="challenge-modal-card">
          <p class="challenge-modal-text">ルームのルール</p>
          <ul class="rules-modal-list" id="rulesModalBody"></ul>
          <button type="button" class="action-button" id="rulesOkBtn">OK</button>
        </div>
      </div>
    `);

    const nameEl = this.overlay.querySelector('#lobbyName');
    if (nameEl) nameEl.textContent = this.name;

    this.renderOptionButtons();
    this.bindOptionClicks();
    this.setMatchUi(false);

    const hostBtn = this.overlay.querySelector('#hostBtn') as HTMLButtonElement;
    const leaveBtn = this.overlay.querySelector('#leaveBtn') as HTMLButtonElement;
    const refreshBtn = this.overlay.querySelector('#refreshRoomsBtn') as HTMLButtonElement;
    const acceptBtn = this.overlay.querySelector('#challengeAcceptBtn') as HTMLButtonElement;
    const declineBtn = this.overlay.querySelector('#challengeDeclineBtn') as HTMLButtonElement;
    const rulesOkBtn = this.overlay.querySelector('#rulesOkBtn') as HTMLButtonElement;
    const hostRulesOkBtn = this.overlay.querySelector('#hostRulesOkBtn') as HTMLButtonElement;
    const hostRulesCancelBtn = this.overlay.querySelector('#hostRulesCancelBtn') as HTMLButtonElement;

    hostBtn.addEventListener('click', () => {
      this.openHostRulesModal();
    });
    hostRulesOkBtn.addEventListener('click', () => {
      void this.confirmHostRoom();
    });
    hostRulesCancelBtn.addEventListener('click', () => {
      this.closeHostRulesModal();
    });

    leaveBtn.addEventListener('click', () => {
      this.leaveMatchToIdle();
    });

    refreshBtn.addEventListener('click', () => {
      void this.refreshRooms();
    });

    this.overlay.querySelector('#roomsList')?.addEventListener('click', (ev) => {
      const rulesBtn = (ev.target as HTMLElement).closest(
        'button[data-rules]'
      ) as HTMLButtonElement | null;
      if (rulesBtn) {
        const roomId = rulesBtn.dataset.rules;
        if (roomId) this.showRoomRules(roomId);
        return;
      }
      const btn = (ev.target as HTMLElement).closest(
        'button[data-challenge]'
      ) as HTMLButtonElement | null;
      if (!btn || btn.disabled) return;
      const roomId = btn.dataset.challenge;
      if (!roomId) return;
      void this.challengeRoom(roomId);
    });

    acceptBtn.addEventListener('click', () => {
      if (!this.network?.isOpen) {
        setOverlayError(this.overlay, '未接続のため承認できません');
        return;
      }
      acceptBtn.disabled = true;
      declineBtn.disabled = true;
      this.acceptSent = true;
      if (!this.network.send('respond_challenge', { accept: true })) {
        this.acceptSent = false;
        acceptBtn.disabled = false;
        declineBtn.disabled = false;
        return;
      }
      // If BUILD never arrives (stuck LOBBY), force start_match once.
      if (this.startWatchTimer != null) clearTimeout(this.startWatchTimer);
      this.startWatchTimer = setTimeout(() => {
        this.startWatchTimer = null;
        if (this.handedOffSocket || !this.acceptSent || !this.network?.isOpen) return;
        const s = this.lastState;
        if (s && s.phase === 'LOBBY' && !s.challenge && s.opponent?.name) {
          this.network.send('start_match', {});
        } else if (s && s.phase === 'LOBBY' && s.challenge) {
          // Accept may have been dropped; retry once.
          this.network.send('respond_challenge', { accept: true });
        }
      }, 800);
    });
    declineBtn.addEventListener('click', () => {
      if (!this.network?.isOpen) {
        setOverlayError(this.overlay, '未接続のため拒否できません');
        return;
      }
      this.network.send('respond_challenge', { accept: false });
    });
    rulesOkBtn.addEventListener('click', () => {
      this.overlay?.querySelector('#rulesModal')?.classList.add('hidden');
    });

    void this.refreshRooms();
    this.roomsTimer = setInterval(() => {
      void this.refreshRooms();
    }, 8000);
  }

  private setMatchUi(inMatch: boolean) {
    const idle = this.overlay?.querySelector('#idlePanel');
    const match = this.overlay?.querySelector('#matchPanel');
    const hint = this.overlay?.querySelector('#lobbyHint');
    idle?.classList.toggle('hidden', inMatch);
    match?.classList.toggle('hidden', !inMatch);
    if (hint) {
      hint.textContent = inMatch
        ? '挑戦が来たら承認すると対局が始まります'
        : '部屋を建てて待つか、対戦待ちルームに挑戦してください';
    }
  }

  private openHostRulesModal() {
    setOverlayError(this.overlay, '');
    this.syncOptionChipActive();
    this.overlay?.querySelector('#hostRulesModal')?.classList.remove('hidden');
  }

  private closeHostRulesModal() {
    this.overlay?.querySelector('#hostRulesModal')?.classList.add('hidden');
  }

  private async confirmHostRoom() {
    const okBtn = this.overlay?.querySelector('#hostRulesOkBtn') as HTMLButtonElement | null;
    const cancelBtn = this.overlay?.querySelector('#hostRulesCancelBtn') as HTMLButtonElement | null;
    if (okBtn) okBtn.disabled = true;
    if (cancelBtn) cancelBtn.disabled = true;
    setOverlayError(this.overlay, '');
    try {
      const sec = secFromPreset(this.draftPreset);
      const roomId = await createRoom({
        totalRounds: this.draftRounds,
        buildTimeoutSec: sec > 0 ? sec : undefined,
      });
      this.closeHostRulesModal();
      this.pendingApplyOptions = true;
      this.connectToRoom(roomId);
    } catch (err) {
      setOverlayError(this.overlay, err instanceof Error ? err.message : String(err));
    } finally {
      if (okBtn) okBtn.disabled = false;
      if (cancelBtn) cancelBtn.disabled = false;
    }
  }

  private async challengeRoom(roomId: string) {
    if (roomId === this.roomId) {
      setOverlayError(this.overlay, 'すでにこのルームにいます');
      return;
    }
    setOverlayError(this.overlay, '');
    this.connectToRoom(roomId);
  }

  private connectToRoom(roomId: string) {
    this.fatalLeave = false;
    this.lastState = null;
    this.roomId = roomId;
    this.setMatchUi(true);
    const box = this.overlay?.querySelector('#playerList');
    if (box) {
      box.innerHTML =
        '<div style="color:#8a9aab;text-align:center;font-size:0.9rem;">接続中...</div>';
    }

    if (this.network) {
      this.network.switchRoom(roomId);
      return;
    }

    // Fresh join: never reuse sessionStorage playerId (avoids same-browser seat takeover).
    this.network = new NetworkManager({ roomId, name: this.name, playerId: null });
    this.network.on('state', (state) => this.onState(state));
    this.network.on('event', (ev) => this.onMatchEvent(ev));
    this.network.on('error', (msg) => this.onNetworkError(msg));
    this.network.on('disconnect', (msg) => this.onNetworkDisconnect(msg));
    this.network.on('reconnecting', (msg) => {
      if (!this.handedOffSocket) showDomToast(msg);
    });
    this.network.connect();
  }

  private leaveMatchToIdle() {
    this.fatalLeave = true;
    this.pendingApplyOptions = false;
    if (this.network?.isOpen) {
      this.network.send('leave', {});
    }
    this.network?.close();
    this.network = null;
    this.roomId = '';
    this.lastState = null;
    this.stopChallengeTick();
    this.closeHostRulesModal();
    const modal = this.overlay?.querySelector('#challengeModal');
    const wait = this.overlay?.querySelector('#challengeWait');
    modal?.classList.add('hidden');
    wait?.classList.add('hidden');
    this.setMatchUi(false);
    setOverlayError(this.overlay, '');
    void this.refreshRooms();
  }

  private onNetworkError(msg: string) {
    const acceptBtn = this.overlay?.querySelector('#challengeAcceptBtn') as HTMLButtonElement | null;
    const declineBtn = this.overlay?.querySelector('#challengeDeclineBtn') as HTMLButtonElement | null;
    if (acceptBtn) acceptBtn.disabled = false;
    if (declineBtn) declineBtn.disabled = false;

    // Only kick back to idle on definitive challenge/join failures (not generic errors).
    if (
      msg.includes('挑戦を拒否') ||
      msg.includes('挑戦の承認がタイムアウト') ||
      msg.includes('ROOM_BUSY') ||
      msg.includes('ROOM_FULL') ||
      msg.includes('room is full') ||
      msg.includes('host is reviewing')
    ) {
      this.fatalLeave = true;
      this.network?.close();
      this.network = null;
      this.roomId = '';
      this.lastState = null;
      this.stopChallengeTick();
      this.setMatchUi(false);
      setOverlayError(this.overlay, msg);
      void this.refreshRooms();
      return;
    }
    setOverlayError(this.overlay, msg);
    if (this.lastState) {
      this.draftPreset = presetFromState(this.lastState);
      this.draftRounds = roundsFromState(this.lastState);
      this.draftWallSorted = this.lastState.wallSorted ?? false;
      this.syncOptionChipActive();
    }
  }

  private onNetworkDisconnect(msg: string) {
    if (this.handedOffSocket || this.fatalLeave) return;
    this.network = null;
    this.roomId = '';
    this.lastState = null;
    this.stopChallengeTick();
    this.setMatchUi(false);
    setOverlayError(this.overlay, msg);
    void this.refreshRooms();
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
    const open = rooms.filter((r) => r.open && r.roomId !== this.roomId);
    this.openRooms = open;
    if (open.length === 0) {
      box.innerHTML = '<div class="login-rooms-empty">いま挑戦できるルームはありません</div>';
      return;
    }
    box.innerHTML = open
      .map((r) => {
        const meta = r.hostName ? `ホスト: ${escapeHtml(r.hostName)}` : '';
        return `
          <div class="login-room-row">
            <div class="login-room-meta">
              <div class="login-room-id">${escapeHtml(r.hostName || 'ホスト')}</div>
              <div class="login-room-sub">${meta}</div>
            </div>
            <div class="login-room-actions">
              <button type="button" class="action-button secondary" data-rules="${escapeHtml(r.roomId)}">ルール</button>
              <button type="button" class="action-button" data-challenge="${escapeHtml(r.roomId)}">挑戦</button>
            </div>
          </div>`;
      })
      .join('');
  }

  private showRoomRules(roomId: string) {
    const room = this.openRooms.find((r) => r.roomId === roomId);
    const modal = this.overlay?.querySelector('#rulesModal');
    const body = this.overlay?.querySelector('#rulesModalBody');
    if (!room || !modal || !body) return;
    const timeoutLabel =
      !room.buildTimeoutSec || room.buildTimeoutSec <= 0
        ? '無限（両者承認）'
        : room.buildTimeoutSec === 120
          ? '2分'
          : room.buildTimeoutSec === 180
            ? '3分'
            : room.buildTimeoutSec === 300
              ? '5分'
              : `${room.buildTimeoutSec}秒`;
    body.innerHTML = `
      <li><span>ホスト</span><span>${escapeHtml(room.hostName || '—')}</span></li>
      <li><span>局数</span><span>${room.totalRounds || '—'}局</span></li>
      <li><span>制限時間</span><span>${escapeHtml(timeoutLabel)}</span></li>
      <li><span>山札理牌</span><span>${room.wallSorted ? 'あり' : 'なし'}</span></li>
    `;
    modal.classList.remove('hidden');
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
      const btn = (ev.target as HTMLElement).closest(
        'button[data-rounds]'
      ) as HTMLButtonElement | null;
      if (!btn || this.syncingOptions) return;
      const n = Number(btn.dataset.rounds);
      if (n !== 4 && n !== 6 && n !== 8 && n !== 10) return;
      this.draftRounds = n;
      this.syncOptionChipActive();
    });

    timeoutBox.addEventListener('click', (ev) => {
      const btn = (ev.target as HTMLElement).closest(
        'button[data-preset]'
      ) as HTMLButtonElement | null;
      if (!btn || this.syncingOptions) return;
      const preset = btn.dataset.preset as TimeoutPreset | undefined;
      if (!preset || !TIMEOUT_OPTIONS.some((o) => o.preset === preset)) return;
      this.draftPreset = preset;
      this.syncOptionChipActive();
    });

    wallBox.addEventListener('click', (ev) => {
      const btn = (ev.target as HTMLElement).closest('button[data-wall]') as HTMLButtonElement | null;
      if (!btn || this.syncingOptions) return;
      this.draftWallSorted = btn.dataset.wall === '1';
      this.syncOptionChipActive();
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
    if (this.lastState?.challenge) return;
    setOverlayError(this.overlay, '');
    this.network.send('set_options', {
      timeoutPreset: this.draftPreset,
      buildTimeoutSec: secFromPreset(this.draftPreset),
      totalRounds: this.draftRounds,
      wallSorted: this.draftWallSorted,
    });
  }

  private onMatchEvent(ev: { phase?: string; kind?: string }) {
    // Backup: if we already applied a non-lobby state, hand off even if a late event arrives.
    // Do NOT send start_match here — accept already starts the match; a nudge races BUILD and errors.
    if (!ev?.phase || ev.phase === 'LOBBY' || this.handedOffSocket) return;
    if (this.lastState?.phase && this.lastState.phase !== 'LOBBY') {
      this.transitionToGame();
    }
  }

  private onState(state: PublicState) {
    this.lastState = state;
    this.roomId = state.roomId || this.roomId;
    this.registry.set('gameState', state);

    // Match started → leave lobby immediately (before DOM renders that look "stuck").
    if (state.phase && state.phase !== 'LOBBY') {
      this.transitionToGame();
      return;
    }

    this.renderPlayers(state);
    this.renderChallenge(state);
    setOverlayError(this.overlay, '');
    void this.refreshRooms();

    if (this.pendingApplyOptions && this.network?.isOpen && !state.challenge) {
      this.pendingApplyOptions = false;
      this.sendOptions();
    }

    // Stuck recovery: both in lobby, no challenge, after host accepted → ask server to start.
    if (
      this.acceptSent &&
      !this.startMatchKickSent &&
      state.phase === 'LOBBY' &&
      !state.challenge &&
      state.opponent?.name &&
      this.network?.isOpen
    ) {
      this.startMatchKickSent = true;
      this.network.send('start_match', {});
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
        text.innerHTML = `${escapeHtml(ch.fromName)}さんから挑戦されました。<br>始めますか？`;
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

    const oppPresent = Boolean(state.opponent?.name);
    let oppHtml: string;
    if (!oppPresent) {
      oppHtml = `
        <div class="lobby-player-row">
          <span>空き席</span>
          <span class="badge empty">対戦待ち</span>
        </div>`;
    } else if (state.challenge) {
      const label = state.challenge.role === 'host' ? '挑戦中' : '承認待ち';
      oppHtml = `
        <div class="lobby-player-row">
          <span>${escapeHtml(state.opponent.name)}</span>
          <span class="badge">${label}</span>
        </div>`;
    } else if (state.opponent.connected === false) {
      oppHtml = `
        <div class="lobby-player-row">
          <span>${escapeHtml(state.opponent.name)}</span>
          <span class="badge empty">再接続待ち</span>
        </div>`;
    } else {
      oppHtml = `
        <div class="lobby-player-row">
          <span>${escapeHtml(state.opponent.name)}</span>
          <span class="badge ready">入室済み</span>
        </div>`;
    }

    box.innerHTML = `
      <div class="lobby-player-row">
        <span>${escapeHtml(state.you.name)}（あなた）</span>
        <span class="badge ready">入室済み</span>
      </div>
      ${oppHtml}
    `;
  }

  private transitionToGame() {
    if (this.handedOffSocket || !this.network) return;
    this.handedOffSocket = true;
    this.acceptSent = false;
    this.startMatchKickSent = false;
    if (this.startWatchTimer != null) {
      clearTimeout(this.startWatchTimer);
      this.startWatchTimer = null;
    }
    this.stopChallengeTick();
    if (this.roomsTimer != null) {
      clearInterval(this.roomsTimer);
      this.roomsTimer = null;
    }

    // Hide lobby DOM immediately so we never look "stuck" on 入室済み.
    unmountMenuOverlay(this.overlay);
    this.overlay = null;

    const network = this.network;
    const socket = network.detach();
    const payload = {
      socket,
      name: this.name,
      roomId: network.getRoomId(),
      playerId: network.getPlayerId(),
      initialState: this.lastState,
    };

    // Phaser ScenePlugin.start shuts down this scene and starts GameScene.
    // Prefer it over game.scene.stop/start from a detached timer (unreliable under WS callbacks).
    try {
      this.scene.start('GameScene', payload);
    } catch (err) {
      console.error('failed to enter GameScene via scene plugin', err);
      try {
        this.game.scene.start('GameScene', payload);
      } catch (err2) {
        console.error('failed to enter GameScene via game.scene', err2);
        this.handedOffSocket = false;
        if (socket && socket.readyState === WebSocket.OPEN) {
          // Re-attach so the user can retry / still receive state.
          this.network = new NetworkManager({
            socket,
            roomId: payload.roomId,
            name: this.name,
            playerId: payload.playerId,
          });
          this.network.on('state', (state) => this.onState(state));
        }
      }
    }
  }

  private onShutdown() {
    this.stopChallengeTick();
    if (this.roomsTimer != null) {
      clearInterval(this.roomsTimer);
      this.roomsTimer = null;
    }
    if (this.startWatchTimer != null) {
      clearTimeout(this.startWatchTimer);
      this.startWatchTimer = null;
    }
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
