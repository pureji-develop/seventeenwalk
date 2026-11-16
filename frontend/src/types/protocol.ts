/** Wire protocol types aligned with SeventeenWalk backend. */

export type Phase =
  | 'LOBBY'
  | 'DEAL'
  | 'DORA_SELECT'
  | 'BUILD'
  | 'WALK'
  | 'ROUND_END'
  | 'GAME_OVER';

export interface TileView {
  id: number;
  kind: string;
}

export interface WelcomePayload {
  playerId: string;
  seat: number;
  roomId: string;
}

export interface YouView {
  seat: number;
  name: string;
  ready: boolean;
  rematch: boolean;
  /** false while this client is in reconnect grace on the server */
  connected?: boolean;
  wall?: TileView[] | null;
  hand?: TileView[] | null;
  pool?: TileView[] | null;
  poolCount: number;
  /** テンパイ時の待ち（BUILD OK後 / WALK） */
  waits?: string[] | null;
  handSubmitted: boolean;
  agariAbandoned: boolean;
  river: TileView[];
  turnCount: number;
}

export interface OpponentView {
  seat: number;
  name: string;
  ready: boolean;
  rematch: boolean;
  /** false while opponent is reconnecting */
  connected?: boolean;
  wallCount: number;
  handCount: number;
  poolCount: number;
  /** 和了時・流局時など公開時のみ */
  hand?: TileView[] | null;
  /** 流局時など残牌公開時のみ */
  pool?: TileView[] | null;
  handSubmitted: boolean;
  agariAbandoned: boolean;
  river: TileView[];
  turnCount: number;
}

export interface WalkView {
  legalTileIds: number[];
  maxTurns: number;
}

export interface YakuDetailView {
  code: string;
  name?: string;
  han: number;
}

export interface RoundResultView {
  result: string;
  winnerSeat: number | null;
  loserSeat?: number | null;
  winningTile?: TileView | null;
  hand?: TileView[] | null;
  waits?: string[] | null;
  yaku?: string[] | null;
  yakuDetails?: YakuDetailView[] | null;
  han?: number;
  limit?: string;
  yakumanCount?: number;
  pointsAwarded?: number;
  handPoints?: number;
  scoreDelta?: [number, number];
  doraIndicator?: TileView | null;
  dora?: string[] | null;
  acks: [boolean, boolean];
}

export interface PublicState {
  roomId: string;
  phase: Phase | string;
  round: number;
  totalRounds: number;
  leadSeat: number;
  toAct: number;
  scores: [number, number];
  doraIndicator: TileView | null;
  dora: string[];
  you: YouView;
  opponent: OpponentView;
  sideWallCount: number;
  sideWall?: TileView[] | null;
  walk: WalkView | null;
  roundResult: RoundResultView | null;
  winnerSeat: number | null;
  buildTimeoutSec?: number;
  buildEndsAtMs?: number;
  buildRemainingSec?: number;
  /** When true, BUILD deadline forces submit on the server. */
  buildTimerEnabled?: boolean;
  /** 山札を理牌表示する */
  wallSorted?: boolean;
  /** 制限時間なし（両者OKまで） */
  buildInfinite?: boolean;
  /** LOBBY中の挑戦承認待ち（ホスト承認ポップアップ用） */
  challenge?: ChallengeView | null;
}

export interface ChallengeView {
  fromName: string;
  fromSeat: number;
  expiresAtMs: number;
  /** "host" | "challenger" */
  role: string;
}

export interface EventPayload {
  kind: string;
  seat?: number | null;
  phase?: string;
  tile?: TileView | null;
  scores?: [number, number];
  extra?: Record<string, unknown>;
}

export interface ServerEnvelope {
  type: string;
  seq?: number;
  payload: unknown;
}
