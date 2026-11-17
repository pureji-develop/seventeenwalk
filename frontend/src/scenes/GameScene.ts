import * as Phaser from 'phaser';
import { NetworkManager } from '../network/NetworkManager';
import type { EventPayload, PublicState, TileView } from '../types/protocol';
import {
  FONT_DISPLAY,
  FONT_UI,
  PHASER_COLOR,
  TILE_SIZE_SCALE,
  UI_COLOR,
  uiFont,
  uiSpace,
} from '../constants/theme';
import {
  buildYakuRows,
  formatLimit,
  formatPoints,
  formatPointsDisplay,
  ronPointsFromLimit,
} from '../utils/yaku';
import { showDomToast } from '../utils/menuOverlay';
import { sortTilesByKind, TILE_DISPLAY_H, TILE_DISPLAY_W, TILE_FRAME_H, TILE_FRAME_W, TILE_GAP } from '../utils/tiles';
import { BuildBoard } from '../game/buildBoard';
import {
  addTileSprite as addTileSpriteTo,
  drawFaceDownRow as drawFaceDownRowTo,
  drawRiverGrid as drawRiverGridTo,
  drawTileRowCentered as drawTileRowCenteredTo,
  drawWaitBottomRight,
  riverBlockHeight as riverBlockHeightOf,
} from '../game/tileSprites';

interface GameSceneData {
  socket: WebSocket | null;
  name: string;
  roomId: string;
  playerId?: string | null;
  /** Snapshot at handoff so we don't rely only on registry. */
  initialState?: PublicState | null;
}

export class GameScene extends Phaser.Scene {
  private network!: NetworkManager;
  private name = '';
  private roomId = '';
  private state: PublicState | null = null;
  /** BUILD: ordered selection (理牌順を保持). */
  private selectedBuildIds: number[] = [];
  /** BUILD: 選択中13枚のプレビュー待ち */
  private previewWaits: string[] = [];
  private previewWaitsKey = '';
  /** 山牌の上下反転（tileId → 逆さまか）。理牌しない山の見た目を安定させる. */
  private wallFlipById = new Map<number, boolean>();
  private toastText!: Phaser.GameObjects.Text;
  private buildTimerText!: Phaser.GameObjects.Text;
  private root!: Phaser.GameObjects.Container;
  private buildTick?: Phaser.Time.TimerEvent;
  /** ロン前の牌確認タメ（打牌を見せてから「ロン」） */
  private ronHoldDone = false;
  private ronHoldTimer?: Phaser.Time.TimerEvent;
  /** ロン宣言演出後に結果画面を出す */
  private ronSplashDone = false;
  private ronSplashTimer?: Phaser.Time.TimerEvent;
  /** ロン結果モーダル ↔ 盤面表示のトグル */
  private ronBoardPeek = false;
  /** 流局宣言の大きな文字を消したあと */
  private drawSplashDone = false;
  private drawSplashTimer?: Phaser.Time.TimerEvent;
  /** BUILD 入室前の「n局目」モーダル */
  private roundIntroDone = false;
  private roundIntroTimer?: Phaser.Time.TimerEvent;
  private lastPhase = '';
  private buildBoard: BuildBoard | null = null;
  private previewDebounce?: Phaser.Time.TimerEvent;

  constructor() {
    super('GameScene');
  }

  init(data: GameSceneData) {
    this.name = data.name;
    this.roomId = data.roomId;
    this.selectedBuildIds = [];
    this.previewWaits = [];
    this.previewWaitsKey = '';
    this.wallFlipById.clear();
    this.roundIntroDone = false;
    this.lastPhase = '';
    const handed = data.initialState ?? null;
    if (handed) {
      this.registry.set('gameState', handed);
      this.state = handed;
    } else {
      this.state = (this.registry.get('gameState') as PublicState) || null;
    }
    this.buildBoard = null;

    this.network = new NetworkManager({
      socket: data.socket ?? undefined,
      roomId: data.roomId,
      name: data.name,
      // Prefer handoff id; otherwise sessionStorage for mid-match reload.
      playerId: data.playerId === undefined ? undefined : data.playerId,
    });
    if (!data.socket) {
      this.network.connect();
    }
  }

  create() {
    this.events.once('shutdown', this.onShutdown, this);

    this.add.rectangle(0, 0, this.scale.width, this.scale.height, PHASER_COLOR.table).setOrigin(0);

    this.root = this.add.container(0, 0);
    this.buildBoard = new BuildBoard(this, this.root);

    this.toastText = this.add
      .text(this.scale.width / 2, uiSpace(20), '', {
        fontFamily: FONT_UI,
        fontSize: uiFont(18),
        color: UI_COLOR.gold,
        backgroundColor: '#071f18cc',
        padding: { x: uiSpace(12), y: uiSpace(6) },
      })
      .setOrigin(0.5, 0)
      .setDepth(20)
      .setAlpha(0);

    this.buildTimerText = this.add
      .text(this.scale.width / 2, uiSpace(72), '', {
        fontFamily: FONT_UI,
        fontSize: uiFont(15),
        fontStyle: 'bold',
        color: UI_COLOR.gold,
        backgroundColor: '#071f18aa',
        padding: { x: uiSpace(10), y: uiSpace(4) },
      })
      .setOrigin(0.5, 0)
      .setDepth(19)
      .setVisible(false);

    this.buildTick = this.time.addEvent({
      delay: 250,
      loop: true,
      callback: () => this.refreshBuildTimerLabel(),
    });

    this.network.on('state', (s) => {
      const prevPhase = this.state?.phase ?? this.lastPhase;
      this.state = s;
      this.registry.set('gameState', s);
      if (s.phase !== 'BUILD') {
        this.selectedBuildIds = [];
        this.previewWaits = [];
        this.previewWaitsKey = '';
        this.wallFlipById.clear();
      } else if (s.wallSorted) {
        this.wallFlipById.clear();
      } else {
        this.ensureWallFlips(s.you.wall ?? []);
      }
      if (s.phase === 'BUILD' && s.you.handSubmitted && Array.isArray(s.you.waits)) {
        this.previewWaits = [...s.you.waits];
      }

      // ロン時: 0.8〜1.2秒盤面確認 →「ロン」2秒 → 結果画面（振り込み牌の黄ハイライトはロン表示後）
      // 流局時は捨て合いレイアウトのまま「流局」＋全牌公開（結果パネルへ切り替えない）
      if (s.phase === 'ROUND_END' && s.roundResult?.result === 'RON') {
        if (prevPhase !== 'ROUND_END') {
          this.ronHoldDone = false;
          this.ronSplashDone = false;
          this.ronBoardPeek = false;
          this.beginRonHold();
          this.lastPhase = s.phase;
          return;
        }
      } else if (s.phase === 'ROUND_END' && s.roundResult?.result === 'DRAW') {
        if (prevPhase !== 'ROUND_END') {
          this.drawSplashDone = false;
          this.beginDrawSplash();
        }
      } else {
        this.clearRonHoldTimer();
        this.clearRonSplashTimer();
        this.clearDrawSplashTimer();
        this.ronHoldDone = false;
        this.ronSplashDone = false;
        this.drawSplashDone = false;
        this.ronBoardPeek = false;
      }

      // BUILD 入室時は「n局目」を2秒表示
      if (s.phase === 'BUILD' && prevPhase !== 'BUILD') {
        this.roundIntroDone = false;
        this.beginRoundIntro();
      } else if (s.phase !== 'BUILD') {
        this.clearRoundIntroTimer();
        this.roundIntroDone = false;
      }

      this.lastPhase = s.phase;
      this.redraw();
      this.refreshBuildTimerLabel();
      this.schedulePreviewWaits(s);
    });
    this.network.on('waits', (waits) => {
      this.previewWaits = Array.isArray(waits) ? waits : [];
      if (this.state?.phase === 'BUILD' && !this.state.you.handSubmitted) {
        this.refreshBuildBoard();
        this.refreshBuildTimerLabel();
      }
    });
    this.network.on('event', (ev) => this.onEvent(ev));
    this.network.on('error', (msg) => {
      // 待ちプレビュー由来の失敗は再送可能にしてトーストは出さない
      if (
        this.state?.phase === 'BUILD' &&
        !this.state.you.handSubmitted &&
        /preview_waits|previewWaits|invalid payload|illegal action|unknown type/i.test(msg)
      ) {
        this.previewWaitsKey = '';
        return;
      }
      showDomToast(msg, { tone: 'error' });
    });
    this.network.on('reconnecting', (msg) => {
      showDomToast(msg);
    });
    this.network.on('disconnect', (msg) => {
      this.scene.start('LoginScene', { error: msg });
    });

    this.redraw();
    this.refreshBuildTimerLabel();
    if (this.state?.phase === 'BUILD') {
      this.roundIntroDone = false;
      this.beginRoundIntro();
    }
    this.schedulePreviewWaits(this.state);
  }

  private schedulePreviewWaits(s: PublicState | null) {
    this.previewDebounce?.remove(false);
    this.previewDebounce = undefined;
    if (!s || s.phase !== 'BUILD' || s.you?.handSubmitted) return;
    if (!this.roundIntroDone) return;
    if (this.selectedBuildIds.length !== 13) {
      if (this.previewWaits.length || this.previewWaitsKey) {
        this.previewWaits = [];
        this.previewWaitsKey = '';
      }
      return;
    }
    this.previewDebounce = this.time.delayedCall(140, () => {
      this.requestPreviewWaitsIfNeeded(s);
    });
  }

  private requestPreviewWaitsIfNeeded(s: PublicState | null) {
    if (!s || s.phase !== 'BUILD' || s.you?.handSubmitted) return;
    if (!this.roundIntroDone) return;
    if (this.selectedBuildIds.length !== 13) {
      if (this.previewWaits.length || this.previewWaitsKey) {
        this.previewWaits = [];
        this.previewWaitsKey = '';
      }
      return;
    }
    const ids = this.selectedBuildIds.filter((id) => typeof id === 'number' && Number.isFinite(id));
    if (ids.length !== 13) return;
    const key = [...ids].sort((a, b) => a - b).join(',');
    if (key === this.previewWaitsKey) return;
    this.previewWaitsKey = key;
    this.network.send('preview_waits', { tileIds: ids });
  }

  private refreshBuildBoard() {
    const s = this.state;
    if (!s || s.phase !== 'BUILD' || !this.buildBoard) return;
    this.buildBoard.render(
      s,
      this.selectedBuildIds,
      this.previewWaits,
      this.wallFlipById,
      this.roundIntroDone,
      (wall) => this.ensureWallFlips(wall),
      {
        onSelectionChanged: () => {
          this.previewWaitsKey = '';
          this.refreshBuildBoard();
          this.schedulePreviewWaits(this.state);
        },
        onSubmit: (tileIds) => {
          this.network.send('submit_hand', { tileIds });
        },
        onRiipai: () => this.riipaiSelected(s),
        onClear: () => {
          this.selectedBuildIds = [];
          this.previewWaits = [];
          this.previewWaitsKey = '';
          this.refreshBuildBoard();
        },
      }
    );
  }

  private refreshBuildTimerLabel() {
    const s = this.state;
    if (!s || s.phase !== 'BUILD' || !this.buildTimerText) {
      this.buildTimerText?.setVisible(false);
      return;
    }
    const infinite = s.buildInfinite === true || s.buildTimeoutSec === 0;
    const enforced = s.buildTimerEnabled === true;
    if (infinite) {
      this.buildTimerText.setText('制限なし（両者 OK で進行）');
      this.buildTimerText.setColor(UI_COLOR.gold);
      this.buildTimerText.setVisible(true);
      return;
    }
    const limit = s.buildTimeoutSec ?? 180;
    // 強制開始オフ中はカウントダウンしない（期限切れに見えて勝手に進んだと誤解されやすい）
    if (!enforced) {
      const mm = Math.floor(limit / 60);
      const ss = limit % 60;
      const clock = `${mm}:${ss.toString().padStart(2, '0')}`;
      this.buildTimerText.setText(`制限 ${clock}（強制開始オフ・両者 OK で進行）`);
      this.buildTimerText.setColor(UI_COLOR.gold);
      this.buildTimerText.setVisible(true);
      return;
    }
    let remain = s.buildRemainingSec ?? limit;
    if (s.buildEndsAtMs && s.buildEndsAtMs > 0) {
      remain = Math.max(0, Math.ceil((s.buildEndsAtMs - Date.now()) / 1000));
    }
    const mm = Math.floor(remain / 60);
    const ss = remain % 60;
    const clock = `${mm}:${ss.toString().padStart(2, '0')}`;
    this.buildTimerText.setText(`残り ${clock}`);
    this.buildTimerText.setColor(remain <= 30 ? UI_COLOR.accentSoft : UI_COLOR.gold);
    this.buildTimerText.setVisible(true);
  }

  /** ロン直前: 切った牌を見せるタメ（0.8〜1.2秒ランダム） */
  private beginRonHold() {
    this.clearRonHoldTimer();
    this.clearRonSplashTimer();
    this.ronHoldDone = false;
    this.ronSplashDone = false;
    this.redraw();
    this.refreshBuildTimerLabel();
    const holdMs = 800 + Math.random() * 400;
    this.ronHoldTimer = this.time.delayedCall(holdMs, () => {
      this.ronHoldDone = true;
      this.ronHoldTimer = undefined;
      this.beginRonSplash();
    });
  }

  private clearRonHoldTimer() {
    this.ronHoldTimer?.remove(false);
    this.ronHoldTimer = undefined;
  }

  private beginRonSplash() {
    this.clearRonSplashTimer();
    this.ronSplashDone = false;
    this.redraw();
    this.refreshBuildTimerLabel();
    this.ronSplashTimer = this.time.delayedCall(2000, () => {
      this.ronSplashDone = true;
      this.ronSplashTimer = undefined;
      this.redraw();
      this.refreshBuildTimerLabel();
    });
  }

  private clearRonSplashTimer() {
    this.ronSplashTimer?.remove(false);
    this.ronSplashTimer = undefined;
  }

  private beginDrawSplash() {
    this.clearDrawSplashTimer();
    this.drawSplashDone = false;
    this.drawSplashTimer = this.time.delayedCall(2000, () => {
      this.drawSplashDone = true;
      this.drawSplashTimer = undefined;
      this.redraw();
      this.refreshBuildTimerLabel();
    });
  }

  private clearDrawSplashTimer() {
    this.drawSplashTimer?.remove(false);
    this.drawSplashTimer = undefined;
  }

  private beginRoundIntro() {
    this.clearRoundIntroTimer();
    this.roundIntroDone = false;
    this.roundIntroTimer = this.time.delayedCall(2000, () => {
      this.roundIntroDone = true;
      this.roundIntroTimer = undefined;
      this.redraw();
      this.refreshBuildTimerLabel();
    });
  }

  private clearRoundIntroTimer() {
    this.roundIntroTimer?.remove(false);
    this.roundIntroTimer = undefined;
  }

  private onEvent(ev: EventPayload) {
    switch (ev.kind) {
      case 'RON':
        // 中央の「ロン」演出で代替
        break;
      case 'GAME_OVER':
        this.showToast('マッチ終了');
        break;
      case 'OPPONENT_LEFT':
        this.showToast('相手が退室しました');
        break;
      case 'OPPONENT_DISCONNECT':
        this.showToast('相手の接続が切れました。再接続を待っています…');
        break;
      case 'HAND_LOCKED':
        this.showToast('手牌が確定しました');
        break;
      case 'DORA':
        this.showToast('ドラ確定');
        break;
      default:
        break;
    }
  }

  private showToast(msg: string) {
    this.toastText.setText(msg);
    this.tweens.killTweensOf(this.toastText);
    this.toastText.setAlpha(1);
    this.tweens.add({
      targets: this.toastText,
      alpha: 0,
      delay: 1600,
      duration: 450,
    });
  }

  private redraw() {
    this.root.removeAll(true);
    this.buildBoard = new BuildBoard(this, this.root);
    const s = this.state;
    if (!s) {
      this.root.add(
        this.add
          .text(this.scale.width / 2, this.scale.height / 2, '状態を待っています…', {
            fontFamily: FONT_UI,
            fontSize: uiFont(20),
            color: UI_COLOR.ivory,
          })
          .setOrigin(0.5)
      );
      return;
    }

    this.drawHeader(s);

    switch (s.phase) {
      case 'DORA_SELECT':
        // サーバは通常自動選定済み。万一残っていれば待ち表示。
        this.root.add(
          this.add
            .text(this.scale.width / 2, this.scale.height / 2, 'ドラ選定中…', {
              fontFamily: FONT_UI,
              fontSize: uiFont(18),
              color: UI_COLOR.muted,
            })
            .setOrigin(0.5)
        );
        break;
      case 'BUILD':
        this.refreshBuildBoard();
        if (!this.roundIntroDone) {
          this.drawRoundIntro(s);
        }
        this.schedulePreviewWaits(s);
        break;
      case 'WALK':
        this.drawWalk(s);
        break;
      case 'ROUND_END':
        if (s.roundResult?.result === 'RON' && !this.ronHoldDone) {
          this.drawRonHold(s);
        } else if (s.roundResult?.result === 'RON' && !this.ronSplashDone) {
          this.drawRonSplash();
        } else if (s.roundResult?.result === 'DRAW') {
          this.drawDrawReveal(s);
        } else if (s.roundResult?.result === 'RON' && this.ronBoardPeek) {
          this.drawRonBoardPeek(s);
        } else {
          this.drawRoundEnd(s);
        }
        break;
      case 'GAME_OVER':
        this.drawGameOver(s);
        break;
      default:
        this.root.add(
          this.add
            .text(this.scale.width / 2, this.scale.height / 2, `phase: ${s.phase}`, {
              fontFamily: FONT_UI,
              fontSize: uiFont(18),
              color: UI_COLOR.muted,
            })
            .setOrigin(0.5)
        );
    }
  }

  private drawHeader(s: PublicState) {
    const youScore = s.scores[s.you.seat] ?? 0;
    const oppScore = s.scores[s.opponent.seat] ?? 0;

    // 左上: ドラ表示牌のみ
    let headerLeft = uiSpace(18);
    if (s.doraIndicator) {
      this.root.add(
        this.add.text(headerLeft, uiSpace(8), 'ドラ', {
          fontFamily: FONT_UI,
          fontSize: uiFont(11),
          color: UI_COLOR.muted,
        })
      );
      this.addTileSprite(
        headerLeft + uiSpace(22),
        uiSpace(40),
        s.doraIndicator,
        0.72 * TILE_SIZE_SCALE
      ).setOrigin(0.5);
      headerLeft = uiSpace(78);
    }

    this.root.add(
      this.add
        .text(headerLeft, uiSpace(8), '十七歩', {
          fontFamily: FONT_DISPLAY,
          fontSize: uiFont(20),
          color: UI_COLOR.ivory,
        })
        .setOrigin(0, 0)
    );
    this.root.add(
      this.add.text(
        headerLeft,
        uiSpace(36),
        `${this.name} · ${this.roomId} · 第${s.round}/${s.totalRounds}局  ${this.phaseLabel(s)}`,
        {
          fontFamily: FONT_UI,
          fontSize: uiFont(12),
          color: UI_COLOR.muted,
        }
      )
    );

    this.drawHeaderScoreBadges(s.you.name, s.opponent.name || '相手', youScore, oppScore);

    if (s.you.agariAbandoned) {
      this.root.add(
        this.add
          .text(this.scale.width / 2, uiSpace(12), 'あがり放棄中（全待ち）', {
            fontFamily: FONT_UI,
            fontSize: uiFont(13),
            color: UI_COLOR.accentSoft,
            backgroundColor: '#3a1820aa',
            padding: { x: uiSpace(8), y: uiSpace(3) },
          })
          .setOrigin(0.5, 0)
      );
    }
    if (s.opponent?.name && s.opponent.connected === false) {
      this.root.add(
        this.add
          .text(this.scale.width / 2, uiSpace(44), '相手の再接続を待っています…', {
            fontFamily: FONT_UI,
            fontSize: uiFont(14),
            color: UI_COLOR.gold,
            backgroundColor: '#071f18cc',
            padding: { x: uiSpace(10), y: uiSpace(4) },
          })
          .setOrigin(0.5, 0)
      );
    }
  }

  /** 右上: 名前1行目・得点2行目の2列バッジ（— は上下同じ位置） */
  private drawHeaderScoreBadges(
    youName: string,
    oppName: string,
    youScore: number,
    oppScore: number
  ) {
    const marginRight = uiSpace(18);
    const dashGap = uiSpace(6);
    const padX = uiSpace(8);
    const padY = uiSpace(4);
    const innerGap = uiSpace(2);
    const topY = uiSpace(8);
    const dashChar = '—';

    const nameStyle: Phaser.Types.GameObjects.Text.TextStyle = {
      fontFamily: FONT_UI,
      fontSize: uiFont(13),
      color: UI_COLOR.ivory,
    };
    const scoreStyle: Phaser.Types.GameObjects.Text.TextStyle = {
      fontFamily: FONT_UI,
      fontSize: uiFont(14),
      fontStyle: 'bold',
      color: UI_COLOR.gold,
    };
    const dashStyle: Phaser.Types.GameObjects.Text.TextStyle = {
      fontFamily: FONT_UI,
      fontSize: uiFont(13),
      color: UI_COLOR.muted,
    };

    const youScoreStr = formatPoints(youScore);
    const oppScoreStr = formatPoints(oppScore);

    const measure = (text: string, style: Phaser.Types.GameObjects.Text.TextStyle) => {
      const t = this.add.text(0, 0, text, style);
      const size = { w: t.width, h: t.height };
      t.destroy();
      return size;
    };

    const youNameM = measure(youName, nameStyle);
    const oppNameM = measure(oppName, nameStyle);
    const youScoreM = measure(youScoreStr, scoreStyle);
    const oppScoreM = measure(oppScoreStr, scoreStyle);
    const dashM = measure(dashChar, dashStyle);

    const leftColW = Math.max(youNameM.w, youScoreM.w);
    const rightColW = Math.max(oppNameM.w, oppScoreM.w);
    const rowH = youNameM.h;
    const badgeH = padY * 2 + rowH + innerGap + youScoreM.h;
    const leftBadgeW = leftColW + padX * 2;
    const rightBadgeW = rightColW + padX * 2;

    const totalW = leftBadgeW + dashGap + dashM.w + dashGap + rightBadgeW;
    const blockRight = this.scale.width - marginRight;
    const blockLeft = blockRight - totalW;

    const leftBadgeX = blockLeft;
    const rightBadgeX = blockLeft + leftBadgeW + dashGap + dashM.w + dashGap;
    const dashX = blockLeft + leftBadgeW + dashGap + dashM.w / 2;

    const nameY = topY + padY;
    const scoreY = nameY + rowH + innerGap;

    const badge = this.add.graphics();
    badge.fillStyle(PHASER_COLOR.panel, 0.78);
    badge.fillRoundedRect(leftBadgeX, topY, leftBadgeW, badgeH, 6);
    badge.fillRoundedRect(rightBadgeX, topY, rightBadgeW, badgeH, 6);
    badge.lineStyle(1, 0xe8eef4, 0.14);
    badge.strokeRoundedRect(leftBadgeX, topY, leftBadgeW, badgeH, 6);
    badge.strokeRoundedRect(rightBadgeX, topY, rightBadgeW, badgeH, 6);
    this.root.add(badge);

    const leftCenterX = leftBadgeX + leftBadgeW / 2;
    const rightCenterX = rightBadgeX + rightBadgeW / 2;

    this.root.add(this.add.text(leftCenterX, nameY, youName, nameStyle).setOrigin(0.5, 0));
    this.root.add(this.add.text(leftCenterX, scoreY, youScoreStr, scoreStyle).setOrigin(0.5, 0));
    this.root.add(this.add.text(rightCenterX, nameY, oppName, nameStyle).setOrigin(0.5, 0));
    this.root.add(this.add.text(rightCenterX, scoreY, oppScoreStr, scoreStyle).setOrigin(0.5, 0));
    this.root.add(
      this.add.text(dashX, topY + badgeH / 2, dashChar, dashStyle).setOrigin(0.5, 0.5)
    );
  }

  private phaseLabel(s: PublicState): string {
    switch (s.phase) {
      case 'DORA_SELECT':
        return 'ドラ選定';
      case 'BUILD':
        return '手牌編成';
      case 'WALK':
        return '捨て合い';
      case 'ROUND_END':
        return '局結果';
      case 'GAME_OVER':
        return 'マッチ終了';
      default:
        return String(s.phase);
    }
  }

  /** BUILD 入室前: 「n局目（全m局）」を2秒表示 */
  private drawRoundIntro(s: PublicState) {
    const cx = this.scale.width / 2;
    const cy = this.scale.height / 2;
    const dim = this.add.rectangle(cx, cy, this.scale.width, this.scale.height, PHASER_COLOR.dim, 0.55);
    this.root.add(dim);

    const panel = this.add.graphics();
    const pw = uiSpace(320);
    const ph = uiSpace(120);
    panel.fillStyle(PHASER_COLOR.panel, 0.95);
    panel.fillRoundedRect(cx - pw / 2, cy - ph / 2, pw, ph, 16);
    panel.lineStyle(1, 0xe8eef4, 0.14);
    panel.strokeRoundedRect(cx - pw / 2, cy - ph / 2, pw, ph, 16);
    this.root.add(panel);

    const n = s.round ?? 1;
    const m = s.totalRounds ?? 4;
    this.root.add(
      this.add
        .text(cx, cy - uiSpace(10), `${n}局目`, {
          fontFamily: FONT_DISPLAY,
          fontSize: uiFont(42),
          color: UI_COLOR.ivory,
        })
        .setOrigin(0.5)
    );
    this.root.add(
      this.add
        .text(cx, cy + uiSpace(28), `（全${m}局）`, {
          fontFamily: FONT_UI,
          fontSize: uiFont(18),
          color: UI_COLOR.gold,
        })
        .setOrigin(0.5)
    );
  }

  /** 山牌ごとの上下反転を一度だけ決める（再描画でチラつかない）. */
  private ensureWallFlips(wall: TileView[]) {
    for (const t of wall) {
      if (!this.wallFlipById.has(t.id)) {
        this.wallFlipById.set(t.id, Math.random() < 0.5);
      }
    }
  }

  /** 取得中の牌だけ kind 順に並べ替え（理牌）。山は並べ替えない. */
  private riipaiSelected(s: PublicState) {
    const wallById = new Map((s.you.wall ?? []).map((t) => [t.id, t]));
    const tiles = this.selectedBuildIds
      .map((id) => wallById.get(id))
      .filter((t): t is TileView => Boolean(t));
    this.selectedBuildIds = sortTilesByKind(tiles).map((t) => t.id);
    this.previewWaitsKey = '';
    this.refreshBuildBoard();
    this.schedulePreviewWaits(this.state);
  }

  private drawWalk(s: PublicState) {
    const myTurn = s.toAct === s.you.seat;
    const legal = new Set(s.walk?.legalTileIds ?? []);
    const w = this.scale.width;
    const h = this.scale.height;
    const riverScale = 0.66 * TILE_SIZE_SCALE;
    const oppHandScale = 0.62 * TILE_SIZE_SCALE;
    const handScale = 0.78 * TILE_SIZE_SCALE;
    const riverTh = Math.round(TILE_FRAME_H * riverScale);
    const oppHandTh = Math.round(TILE_FRAME_H * oppHandScale);

    this.root.add(
      this.add
        .text(w / 2, uiSpace(70), myTurn ? 'あなたの番 — 捨て候補から打牌' : '相手の番…', {
          fontFamily: FONT_UI,
          fontSize: uiFont(15),
          color: myTurn ? UI_COLOR.accentSoft : UI_COLOR.muted,
        })
        .setOrigin(0.5)
    );

    // --- 上段: 相手手牌（ラベルなし） ---
    const oppHandY = uiSpace(100);
    const oppRevealed = s.opponent.hand && s.opponent.hand.length > 0;
    if (oppRevealed) {
      this.drawTileRowCentered(sortTilesByKind(s.opponent.hand!), oppHandY, oppHandScale);
    } else {
      this.drawFaceDownRow(Math.max(s.opponent.handCount, 13), oppHandY, oppHandScale);
    }

    // --- 下段: 捨て候補 → 手牌 ---
    const handY = h - uiSpace(42);
    const poolRowGap = TILE_DISPLAY_H + uiSpace(6);
    const poolBottomY = handY - uiSpace(64);
    const poolTopY = poolBottomY - poolRowGap;

    // --- 中央: 河を相手手牌〜捨て候補のあいだで上下中央揃え ---
    const riverBlock = this.riverBlockHeight(riverScale);
    const riverGap = uiSpace(10);
    const riverPairH = riverBlock * 2 + riverGap;
    const zoneTop = oppHandY + oppHandTh / 2;
    const zoneBottom = poolTopY - TILE_DISPLAY_H / 2;
    const zoneMid = (zoneTop + zoneBottom) / 2;
    const oppRiverTopY = zoneMid - riverPairH / 2 + riverTh / 2;

    this.drawRiverGrid(s.opponent.river ?? [], oppRiverTopY, {
      scale: riverScale,
      side: 'opponent',
    });
    this.drawRiverGrid(s.you.river ?? [], oppRiverTopY + riverBlock + riverGap, {
      scale: riverScale,
      side: 'self',
    });

    const pool = sortTilesByKind(s.you.pool ?? []);
    const row1Count = Math.ceil(pool.length / 2);
    const row1 = pool.slice(0, row1Count);
    const row2 = pool.slice(row1Count);
    this.drawPoolRow(row1, poolTopY, myTurn, legal);
    this.drawPoolRow(row2, poolBottomY, myTurn, legal);

    const waitKinds = Array.isArray(s.you.waits) ? s.you.waits : [];
    drawWaitBottomRight(this, this.root, waitKinds, {
      y: h - uiSpace(28),
      scale: 0.62 * TILE_SIZE_SCALE,
    });
    this.drawTileRowCentered(sortTilesByKind(s.you.hand ?? []), handY, handScale);
  }

  private drawPoolRow(
    tiles: TileView[],
    y: number,
    myTurn: boolean,
    legal: Set<number>
  ) {
    if (tiles.length === 0) return;
    const gap = TILE_GAP + 2;
    const totalW = tiles.length * (TILE_DISPLAY_W + gap) - gap;
    const startX = (this.scale.width - totalW) / 2 + TILE_DISPLAY_W / 2;
    tiles.forEach((t, i) => {
      const x = startX + i * (TILE_DISPLAY_W + gap);
      const can = myTurn && legal.has(t.id);
      const img = this.addTileSprite(x, y, t);
      if (!can) {
        img.setAlpha(0.45);
      } else {
        img.setInteractive({ useHandCursor: true });
        img.on('pointerover', () => img.setY(y - 8));
        img.on('pointerout', () => img.setY(y));
        img.on('pointerdown', () => {
          this.network.send('discard_tile', { tileId: t.id });
        });
      }
    });
  }

  /**
   * ロン直前: 捨て合い盤面のまま（振り込み牌のハイライトはまだ出さない）。
   */
  private drawRonHold(s: PublicState) {
    const h = this.scale.height;
    const riverScale = 0.66 * TILE_SIZE_SCALE;
    const oppHandScale = 0.62 * TILE_SIZE_SCALE;
    const handScale = 0.78 * TILE_SIZE_SCALE;
    const riverTh = Math.round(TILE_FRAME_H * riverScale);
    const oppHandTh = Math.round(TILE_FRAME_H * oppHandScale);

    const oppHandY = uiSpace(100);
    this.drawFaceDownRow(Math.max(s.opponent.handCount, 13), oppHandY, oppHandScale);

    const handY = h - uiSpace(42);
    const poolRowGap = TILE_DISPLAY_H + uiSpace(6);
    const poolBottomY = handY - uiSpace(64);
    const poolTopY = poolBottomY - poolRowGap;

    const riverBlock = this.riverBlockHeight(riverScale);
    const riverGap = uiSpace(10);
    const riverPairH = riverBlock * 2 + riverGap;
    const zoneTop = oppHandY + oppHandTh / 2;
    const zoneBottom = poolTopY - TILE_DISPLAY_H / 2;
    const zoneMid = (zoneTop + zoneBottom) / 2;
    const oppRiverTopY = zoneMid - riverPairH / 2 + riverTh / 2;

    this.drawRiverGrid(s.opponent.river ?? [], oppRiverTopY, {
      scale: riverScale,
      side: 'opponent',
    });
    this.drawRiverGrid(s.you.river ?? [], oppRiverTopY + riverBlock + riverGap, {
      scale: riverScale,
      side: 'self',
    });

    const pool = sortTilesByKind(s.you.pool ?? []);
    const row1Count = Math.ceil(pool.length / 2);
    const row1 = pool.slice(0, row1Count);
    const row2 = pool.slice(row1Count);
    this.drawPoolRow(row1, poolTopY, false, new Set());
    this.drawPoolRow(row2, poolBottomY, false, new Set());

    this.drawTileRowCentered(sortTilesByKind(s.you.hand ?? []), handY, handScale);
  }

  private drawRonSplash() {
    const cx = this.scale.width / 2;
    const cy = this.scale.height / 2;

    // 薄い暗幕のうえに、文字だけ（帯や枠なし）
    const dim = this.add.rectangle(cx, cy, this.scale.width, this.scale.height, PHASER_COLOR.dim, 0.4);
    this.root.add(dim);

    const label = this.add
      .text(cx, cy, 'ロン', {
        fontFamily: FONT_DISPLAY,
        fontSize: uiFont(120),
        color: '#c45a4a',
      })
      .setOrigin(0.5)
      .setAlpha(0);
    this.root.add(label);

    this.tweens.add({
      targets: label,
      alpha: 1,
      duration: 320,
      ease: 'Sine.easeOut',
    });
  }

  /**
   * 流局: 結果パネルへ切り替えず、捨て合いレイアウトのまま全牌開示。
   * 中央に「流局」のみ出し、両者 OK で次局へ。
   */
  private drawDrawReveal(s: PublicState) {
    this.drawEndBoardReveal(s);

    // 中央に「流局」のみ（結果パネルなし）。2秒後は小さく残す
    const w = this.scale.width;
    const h = this.scale.height;
    const cx = w / 2;
    const cy = h / 2 - 10;
    if (!this.drawSplashDone) {
      const dim = this.add.rectangle(cx, cy, w, h, PHASER_COLOR.dim, 0.28);
      this.root.add(dim);
      const label = this.add
        .text(cx, cy, '流局', {
          fontFamily: FONT_DISPLAY,
          fontSize: uiFont(120),
          color: '#9aa8b8',
        })
        .setOrigin(0.5)
        .setAlpha(0);
      this.root.add(label);
      this.tweens.add({
        targets: label,
        alpha: 1,
        duration: 320,
        ease: 'Sine.easeOut',
      });
    } else {
      this.root.add(
        this.add
          .text(cx, 88, '流局', {
            fontFamily: FONT_DISPLAY,
            fontSize: uiFont(28),
            color: UI_COLOR.ivory,
          })
          .setOrigin(0.5)
      );
    }

    const rr = s.roundResult;
    const acked = rr?.acks?.[s.you.seat] ?? false;
    this.addActionButton(
      w - 110,
      h - 40,
      acked ? '相手の OK 待ち…' : 'OK',
      () => {
        if (!acked) this.network.send('ack_round', {});
      },
      !acked
    );
  }

  /** ロン結果: 目アイコンで盤面確認（結果モーダルとトグル） */
  private drawRonBoardPeek(s: PublicState) {
    this.drawEndBoardReveal(s);
    this.root.add(
      this.add
        .text(this.scale.width / 2, 88, 'ロン', {
          fontFamily: FONT_DISPLAY,
          fontSize: uiFont(22),
          color: '#c45a4a',
        })
        .setOrigin(0.5)
    );
    this.addRonEyeToggle(true);

    const rr = s.roundResult;
    const acked = rr?.acks?.[s.you.seat] ?? false;
    this.addActionButton(
      this.scale.width - 110,
      this.scale.height - 40,
      acked ? '相手の確認待ち…' : '次へ（確認）',
      () => {
        if (!acked) this.network.send('ack_round', {});
      },
      !acked
    );
  }

  /** 局終了時の盤面開示レイアウト（流局 / ロン peek 共通） */
  private drawEndBoardReveal(s: PublicState) {
    const h = this.scale.height;
    const scaleSmall = 0.55 * TILE_SIZE_SCALE;
    const riverScale = 0.62 * TILE_SIZE_SCALE;
    const riverTh = Math.round(TILE_FRAME_H * riverScale);
    const rr = s.roundResult;
    const winId = rr?.result === 'RON' ? rr.winningTile?.id : undefined;
    const loserSeat = rr?.result === 'RON' ? rr.loserSeat : undefined;

    const oppHandY = uiSpace(90);
    let oppHand = s.opponent.hand?.length ? s.opponent.hand : [];
    if (
      !oppHand.length &&
      rr?.result === 'RON' &&
      rr.winnerSeat === s.opponent.seat &&
      rr.hand?.length
    ) {
      oppHand = rr.hand;
    }
    if (oppHand.length) {
      this.drawTileRowCentered(sortTilesByKind(oppHand), oppHandY, scaleSmall);
    } else {
      this.drawFaceDownRow(Math.max(s.opponent.handCount, 13), oppHandY, scaleSmall);
    }

    const oppPoolY = oppHandY + uiSpace(32);
    const oppPool = sortTilesByKind(s.opponent.pool ?? []);
    if (oppPool.length) {
      this.drawTileRowCentered(oppPool, oppPoolY, 0.48 * TILE_SIZE_SCALE);
    }

    const handY = h - uiSpace(40);
    const myPoolY = handY - uiSpace(52);
    const myPoolScale = 0.48 * TILE_SIZE_SCALE;
    const myPoolTh = Math.round(TILE_FRAME_H * myPoolScale);

    const riverBlock = this.riverBlockHeight(riverScale);
    const riverGap = 12;
    const riverPairH = riverBlock * 2 + riverGap;
    const zoneTop = oppPoolY + Math.round(TILE_FRAME_H * myPoolScale) / 2 + uiSpace(6);
    const zoneBottom = myPoolY - myPoolTh / 2;
    const zoneMid = (zoneTop + zoneBottom) / 2;
    const oppRiverTopY = zoneMid - riverPairH / 2 + riverTh / 2;

    this.drawRiverGrid(s.opponent.river ?? [], oppRiverTopY, {
      scale: riverScale,
      side: 'opponent',
      highlightId: loserSeat === s.opponent.seat ? winId : undefined,
    });
    this.drawRiverGrid(s.you.river ?? [], oppRiverTopY + riverBlock + riverGap, {
      scale: riverScale,
      side: 'self',
      highlightId: loserSeat === s.you.seat ? winId : undefined,
    });

    const myPool = sortTilesByKind(s.you.pool ?? []);
    if (myPool.length) {
      this.drawTileRowCentered(myPool, myPoolY, myPoolScale);
    }

    let myHand = s.you.hand ?? [];
    if (
      !myHand.length &&
      rr?.result === 'RON' &&
      rr.winnerSeat === s.you.seat &&
      rr.hand?.length
    ) {
      myHand = rr.hand;
    }
    this.drawTileRowCentered(sortTilesByKind(myHand), handY, 0.65 * TILE_SIZE_SCALE);
  }

  /** ロン結果: 右上の目アイコン（押下で盤面 ↔ モーダル） */
  private addRonEyeToggle(peeking: boolean) {
    const x = this.scale.width - 36;
    const y = 78;
    const accent = peeking ? 0xd4b06a : 0xe8eef4;
    const fill = peeking ? 0x2a2210 : PHASER_COLOR.panel;

    const hit = this.add.circle(x, y, 18, fill, 0.92);
    hit.setStrokeStyle(1.5, accent, peeking ? 0.95 : 0.45);
    hit.setInteractive({ useHandCursor: true });
    hit.on('pointerover', () => hit.setStrokeStyle(1.5, accent, 1));
    hit.on('pointerout', () => hit.setStrokeStyle(1.5, accent, peeking ? 0.95 : 0.45));
    hit.on('pointerdown', () => {
      this.ronBoardPeek = !this.ronBoardPeek;
      this.redraw();
    });
    this.root.add(hit);

    const g = this.add.graphics();
    g.lineStyle(2, accent, 0.95);
    g.strokeEllipse(x, y, 20, 11);
    g.fillStyle(accent, 1);
    g.fillCircle(x, y, 3.5);
    this.root.add(g);
  }

  private drawRoundEnd(s: PublicState) {
    const rr = s.roundResult;
    if (!rr) return;

    const youWon = rr.result === 'RON' && rr.winnerSeat === s.you.seat;
    const oppWon = rr.result === 'RON' && rr.winnerSeat === s.opponent.seat;
    const winHand = rr.hand?.length ? rr.hand : [];
    const agari = rr.winningTile ?? null;

    // 勝者側のみ: 手牌＋アガリ牌（ヴィラン=上 / ヒーロー=下）
    if (oppWon && winHand.length) {
      this.drawHandWithAgari(sortTilesByKind(winHand), agari, uiSpace(100), 0.72 * TILE_SIZE_SCALE);
    }

    const panel = this.add.graphics();
    const panelLeft = 100;
    const panelW = this.scale.width - 200;
    const panelTop = oppWon ? uiSpace(145) : youWon ? uiSpace(64) : uiSpace(72);
    const panelH = oppWon || youWon ? uiSpace(340) : uiSpace(360);
    panel.fillStyle(PHASER_COLOR.panel, 0.92);
    panel.fillRoundedRect(panelLeft, panelTop, panelW, panelH, 16);
    panel.lineStyle(1, 0xe8eef4, 0.12);
    panel.strokeRoundedRect(panelLeft, panelTop, panelW, panelH, 16);
    this.root.add(panel);

    const title = rr.result === 'RON' ? '栄和' : '流局';
    this.root.add(
      this.add
        .text(this.scale.width / 2, panelTop + uiSpace(22), title, {
          fontFamily: FONT_DISPLAY,
          fontSize: uiFont(26),
          color: UI_COLOR.ivory,
        })
        .setOrigin(0.5)
    );

    const bodyTop = panelTop + uiSpace(56);
    if (rr.result === 'RON') {
      const limitLabel = formatLimit(rr.limit, rr.yakumanCount);

      // サーバ値が小さすぎる（旧ハーフ点）場合は役から再計算
      let handPts = rr.handPoints ?? 0;
      const fromLimit = ronPointsFromLimit(rr.limit, rr.yakumanCount ?? 0);
      if (handPts < 1000 && fromLimit > 0) {
        handPts = fromLimit;
      }
      let totalPts = rr.pointsAwarded ?? 0;
      if (totalPts < 1000) {
        totalPts = handPts;
      }

      // 2カラム: 左=役一覧 / 右=ドラ・翻・得点・待ち
      const colGap = uiSpace(28);
      const innerPad = uiSpace(28);
      const colW = (panelW - innerPad * 2 - colGap) / 2;
      const leftColX = panelLeft + innerPad;
      const rightColX = panelLeft + innerPad + colW + colGap;
      const rightCx = rightColX + colW / 2;

      // --- 左: 役（行間を広めに） ---
      const yakuRows = buildYakuRows(rr.yakuDetails, rr.yaku, rr.han ?? 0);
      const yakuRowH = uiSpace(28);
      yakuRows.forEach((row, i) => {
        const yy = bodyTop + i * yakuRowH;
        this.root.add(
          this.add.text(leftColX, yy, row.name, {
            fontFamily: FONT_DISPLAY,
            fontSize: uiFont(17),
            color: UI_COLOR.ivory,
          })
        );
        this.root.add(
          this.add
            .text(leftColX + colW, yy, `${row.han}翻`, {
              fontFamily: FONT_DISPLAY,
              fontSize: uiFont(17),
              color: UI_COLOR.gold,
            })
            .setOrigin(1, 0)
        );
      });

      // --- 右: ドラ → 翻 → 得点 → 待ち ---
      let ry = bodyTop;
      ry = this.drawDoraRow(ry, rr.doraIndicator ?? s.doraIndicator, rightCx);
      ry += uiSpace(8);

      const hanLine = limitLabel ? `${rr.han ?? 0}翻　${limitLabel}` : `${rr.han ?? 0}翻`;
      this.root.add(
        this.add
          .text(rightCx, ry, hanLine, {
            fontFamily: FONT_DISPLAY,
            fontSize: uiFont(15),
            color: UI_COLOR.gold,
          })
          .setOrigin(0.5, 0)
      );
      ry += uiSpace(28);

      this.root.add(
        this.add
          .text(rightCx, ry, formatPointsDisplay(totalPts), {
            fontFamily: FONT_DISPLAY,
            fontSize: uiFont(32),
            color: UI_COLOR.accentSoft,
          })
          .setOrigin(0.5, 0)
      );
      ry += uiSpace(40);

      if (rr.waits?.length) {
        this.root.add(
          this.add
            .text(rightCx, ry, '待ち', {
              fontFamily: FONT_DISPLAY,
              fontSize: uiFont(12),
              color: UI_COLOR.muted,
            })
            .setOrigin(0.5, 0)
        );
        ry += uiSpace(22);
        const waitTiles: TileView[] = rr.waits.map((kind, i) => ({ id: -1000 - i, kind }));
        const waitScale = 0.55 * TILE_SIZE_SCALE;
        this.drawTileRowAt(
          waitTiles,
          rightCx,
          ry + Math.round(TILE_FRAME_H * waitScale) / 2,
          waitScale
        );
      }
    } else {
      this.root.add(
        this.add
          .text(this.scale.width / 2, bodyTop, '17巡打ち切り — 得点変動なし', {
            fontFamily: FONT_UI,
            fontSize: uiFont(15),
            color: UI_COLOR.muted,
            wordWrap: { width: this.scale.width - 340 },
            align: 'center',
          })
          .setOrigin(0.5, 0)
      );
    }

    if (youWon && winHand.length) {
      this.drawHandWithAgari(
        sortTilesByKind(winHand),
        agari,
        this.scale.height - uiSpace(48),
        0.72 * TILE_SIZE_SCALE
      );
    }

    const acked = rr.acks[s.you.seat];
    if (rr.result === 'RON') {
      this.addRonEyeToggle(false);
    }
    // 右下に確認ボタン
    this.addActionButton(
      this.scale.width - 110,
      this.scale.height - 40,
      acked ? '相手の確認待ち…' : '次へ（確認）',
      () => {
        if (!acked) this.network.send('ack_round', {});
      },
      !acked
    );
  }

  /** ドラ表示牌の画像のみ（「ドラ」ラベル + 表示牌）。centerX 省略時は画面中央。 */
  private drawDoraRow(
    y: number,
    indicator: TileView | null | undefined,
    centerX?: number
  ): number {
    const cx = centerX ?? this.scale.width / 2;
    const doraScale = 0.55 * TILE_SIZE_SCALE;
    const rowY = y + Math.round(TILE_FRAME_H * doraScale) / 2;
    this.root.add(
      this.add
        .text(cx - uiSpace(28), rowY, 'ドラ', {
          fontFamily: FONT_UI,
          fontSize: uiFont(12),
          color: UI_COLOR.muted,
        })
        .setOrigin(1, 0.5)
    );
    if (indicator) {
      this.addTileSprite(cx + uiSpace(6), rowY, indicator, doraScale);
    } else {
      this.root.add(
        this.add
          .text(cx + uiSpace(6), rowY, '—', {
            fontFamily: FONT_UI,
            fontSize: uiFont(12),
            color: UI_COLOR.muted,
          })
          .setOrigin(0.5)
      );
    }
    return y + Math.round(TILE_FRAME_H * doraScale) + uiSpace(10);
  }

  /** 指定 X 中心に牌を横一列で並べる */
  private drawTileRowAt(tiles: TileView[], centerX: number, y: number, scale: number) {
    if (!tiles.length) return;
    const tw = Math.round(TILE_FRAME_W * scale);
    const gap = TILE_GAP;
    const totalW = tiles.length * (tw + gap) - gap;
    let x = centerX - totalW / 2 + tw / 2;
    tiles.forEach((t) => {
      this.addTileSprite(x, y, t, scale);
      x += tw + gap;
    });
  }

  /** 手牌の右にアガリ牌を離して並べる */
  private drawHandWithAgari(
    hand: TileView[],
    agari: TileView | null,
    y: number,
    scale: number
  ) {
    const tw = Math.round(TILE_FRAME_W * scale);
    const th = Math.round(TILE_FRAME_H * scale);
    const gap = TILE_GAP;
    const agariGap = 20;
    const handW = Math.max(hand.length, 1) * (tw + gap) - gap;
    const totalW = handW + (agari ? agariGap + tw : 0);
    let x = (this.scale.width - totalW) / 2 + tw / 2;
    hand.forEach((t) => {
      this.addTileSprite(x, y, t, scale);
      x += tw + gap;
    });
    if (agari) {
      x += agariGap - gap;
      const glow = this.add.rectangle(x, y, tw + 6, th + 6, 0xd4b06a, 0.35);
      this.root.add(glow);
      this.addTileSprite(x, y, agari, scale);
      this.root.add(
        this.add
          .text(x, y + th / 2 + 8, 'アガリ', {
            fontFamily: FONT_DISPLAY,
            fontSize: uiFont(11),
            color: UI_COLOR.gold,
          })
          .setOrigin(0.5, 0)
      );
    }
  }

  private drawGameOver(s: PublicState) {
    const youScore = s.scores[s.you.seat] ?? 0;
    const oppScore = s.scores[s.opponent.seat] ?? 0;
    let result = '引き分け';
    if (s.winnerSeat === s.you.seat) result = 'あなたの勝ち';
    else if (s.winnerSeat === s.opponent.seat) result = '相手の勝ち';

    this.root.add(
      this.add
        .text(this.scale.width / 2, 200, 'マッチ終了', {
          fontFamily: FONT_DISPLAY,
          fontSize: uiFont(32),
          color: UI_COLOR.ivory,
        })
        .setOrigin(0.5)
    );
    this.root.add(
      this.add
        .text(this.scale.width / 2, 260, result, {
          fontFamily: FONT_UI,
          fontSize: uiFont(22),
          color: UI_COLOR.accentSoft,
        })
        .setOrigin(0.5)
    );
    this.root.add(
      this.add
        .text(this.scale.width / 2, 310, `${formatPoints(youScore)}  —  ${formatPoints(oppScore)}`, {
          fontFamily: FONT_UI,
          fontSize: uiFont(20),
          color: UI_COLOR.ivory,
        })
        .setOrigin(0.5)
    );

    const rematchLabel = s.you.rematch ? '再戦希望済み…' : '再戦する';
    this.addActionButton(this.scale.width / 2 - 110, 420, rematchLabel, () => {
      if (!s.you.rematch) this.network.send('rematch', {});
    }, !s.you.rematch);

    this.addActionButton(this.scale.width / 2 + 110, 420, '退室', () => {
      this.network.send('leave', {});
      this.network.close();
      this.scene.start('LoginScene');
    }, true);
  }

  private riverBlockHeight(scale: number): number {
    return riverBlockHeightOf(scale);
  }

  private drawRiverGrid(
    tiles: TileView[],
    topCenterY: number,
    opts?: {
      highlightId?: number;
      scale?: number;
      side?: 'self' | 'opponent';
    }
  ) {
    drawRiverGridTo(this, this.root, tiles, topCenterY, opts);
  }

  private drawTileRowCentered(tiles: TileView[], y: number, scale = 0.8) {
    drawTileRowCenteredTo(this, this.root, tiles, y, scale);
  }

  private drawFaceDownRow(count: number, y: number, scale = 0.72) {
    drawFaceDownRowTo(this, this.root, count, y, scale);
  }

  private addTileSprite(x: number, y: number, tile: TileView, scale?: number) {
    return addTileSpriteTo(this, this.root, x, y, tile, scale);
  }

  private addActionButton(
    x: number,
    y: number,
    label: string,
    onClick: () => void,
    enabled: boolean
  ) {
    const btn = this.add
      .text(x, y, label, {
        fontFamily: FONT_UI,
        fontSize: uiFont(16),
        fontStyle: 'bold',
        color: enabled ? UI_COLOR.ivory : UI_COLOR.muted,
        backgroundColor: enabled ? UI_COLOR.accent : '#2a4a3c',
        padding: { x: 22, y: 12 },
      })
      .setOrigin(0.5);
    this.root.add(btn);
    if (enabled) {
      btn.setInteractive({ useHandCursor: true });
      btn.on('pointerover', () => btn.setStyle({ backgroundColor: '#f07060' }));
      btn.on('pointerout', () => btn.setStyle({ backgroundColor: UI_COLOR.accent }));
      btn.on('pointerdown', onClick);
    }
    return btn;
  }

  private onShutdown() {
    this.buildTick?.remove(false);
    this.buildTick = undefined;
    this.previewDebounce?.remove(false);
    this.previewDebounce = undefined;
    this.clearRonHoldTimer();
    this.clearRonSplashTimer();
    this.clearDrawSplashTimer();
    this.clearRoundIntroTimer();
    this.network?.close();
  }
}
