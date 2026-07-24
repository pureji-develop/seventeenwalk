# 02. WebSocket プロトコル仕様

## トランスポート

- URL: `GET /ws?roomId={id}&name={name}`（任意: `&playerId={id}` で再接続）
- サブプロトコルなし、テキストフレームで JSON
- ルーム作成: `POST /rooms` → `{ "roomId": "..." }`
- 再接続: 同一 `playerId` で再接続すると席を再利用（対戦中は約45秒の猶予）。`welcome.playerId` をクライアントが保持する

## 共通エンベロープ

### Client → Server

```json
{
  "type": "discard_tile",
  "requestId": "optional-uuid",
  "payload": {}
}
```

| フィールド | 必須 | 説明 |
|------------|------|------|
| `type` | Yes | アクション種別 |
| `requestId` | No | クライアント任意。エラー応答でエコー可能 |
| `payload` | Yes | アクション固有。空オブジェクト可 |

### Server → Client

```json
{
  "type": "state",
  "seq": 42,
  "payload": {}
}
```

| フィールド | 必須 | 説明 |
|------------|------|------|
| `type` | Yes | `welcome` / `state` / `event` / `error` / `pong` |
| `seq` | state/event で Yes | ルーム内単調増加。古い seq は無視 |
| `payload` | Yes | 種別固有 |

## 牌表現

### 種類（kind）

スプライト ID と一致させる。

| 種類 | 表記 |
|------|------|
| 萬子 1–9 | `m1` … `m9` |
| 筒子 1–9 | `p1` … `p9` |
| 索子 1–9 | `s1` … `s9` |
| 字牌 | `ton` `nan` `sha` `pei` `haku` `hatsu` `chun` |

同一種類は山に最大4枚。区別のため局内一意の `tileId`（整数）を付与する。

### ワイヤ上の牌オブジェクト

```json
{ "id": 42, "kind": "m5" }
```

アクションでは主に `id` を送る。表示は `kind`（および `tiles.webp` の座標）を使う。

## Client → Server アクション

### `join`

接続時にクエリで room/name を渡すため、明示 `join` は任意。

```json
{ "type": "join", "payload": { "roomId": "r_abc", "name": "Alice" } }
```

### `ready`（LOBBY）

```json
{ "type": "ready", "payload": { "ready": true } }
```

### `set_options`（LOBBY）

ロビーの局数・BUILD制限・山札理牌を変更する。変更すると両者の Ready が解除される。

```json
{
  "type": "set_options",
  "payload": {
    "timeoutPreset": "180",
    "buildTimeoutSec": 180,
    "totalRounds": 4,
    "wallSorted": false
  }
}
```

| フィールド | 説明 |
|------------|------|
| `timeoutPreset` | `"120"` / `"180"` / `"300"` / `"inf"`（推奨） |
| `buildTimeoutSec` | 互換用。`inf` 時は 0 |
| `totalRounds` | `4` / `6` / `8` / `10`（省略時は現行値） |
| `wallSorted` | 山札を理牌表示するか |

### `select_dora`（互換・通常は未使用）

現行実装では配牌直後にサーバが自動でドラ表示牌を選び、即 `BUILD` へ進む。  
`DORA_SELECT` フェーズと `select_dora` はプロトコル互換のため残している。

```json
{ "type": "select_dora", "payload": { "tileId": 87 } }
```

### `submit_hand`（BUILD）

```json
{
  "type": "submit_hand",
  "payload": { "tileIds": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13] }
}
```

- ちょうど13個、すべて自分の wall に含まれること

### `preview_waits`（BUILD・提出前）

選択中の13枚についてテンパイ待ちを返す。失敗時もクライアント向けに空配列の `waits` を返す（エラートーストにしない）。

```json
{ "type": "preview_waits", "payload": { "tileIds": [/* 13 ids */] } }
```

応答:

```json
{ "type": "waits", "payload": { "waits": ["s8", "s9"] } }
```

### `discard_tile`（WALK）
```json
{ "type": "discard_tile", "payload": { "tileId": 21 } }
```

- 自分の pool に含まれること
- 手番であること

### `ack_round`（ROUND_END）

```json
{ "type": "ack_round", "payload": {} }
```

### `rematch`（GAME_OVER）

```json
{ "type": "rematch", "payload": {} }
```

### `leave`

```json
{ "type": "leave", "payload": {} }
```

### `ping`

```json
{ "type": "ping", "payload": {} }
```

## Server → Client メッセージ

### `welcome`（接続直後・個別）

```json
{
  "type": "welcome",
  "payload": {
    "playerId": "p_xxx",
    "seat": 1,
    "roomId": "r_abc"
  }
}
```

### `state`（メイン・マスク済みフルスナップショット）

```json
{
  "type": "state",
  "seq": 17,
  "payload": {
    "roomId": "r_abc",
    "phase": "WALK",
    "round": 2,
    "totalRounds": 4,
    "leadSeat": 0,
    "toAct": 1,
    "scores": [1, 0],
    "doraIndicator": { "id": 87, "kind": "s3" },
    "dora": ["s4"],
    "you": {
      "seat": 1,
      "name": "Alice",
      "ready": true,
      "rematch": false,
      "connected": true,
      "wall": null,
      "hand": [
        { "id": 1, "kind": "m1" }
      ],
      "pool": [
        { "id": 20, "kind": "p9" }
      ],
      "poolCount": 18,
      "handSubmitted": true,
      "river": [
        { "id": 30, "kind": "nan" }
      ],
      "turnCount": 2
    },
    "opponent": {
      "seat": 0,
      "name": "Bob",
      "ready": true,
      "rematch": false,
      "connected": true,
      "wallCount": 0,
      "handCount": 13,
      "poolCount": 19,
      "handSubmitted": true,
      "river": [
        { "id": 55, "kind": "ton" }
      ],
      "turnCount": 3
    },
    "sideWallCount": 34,
    "sideWall": null,
    "walk": {
      "legalTileIds": [20, 22, 25],
      "maxTurns": 17
    },
    "roundResult": null,
    "winnerSeat": null
  }
}
```

#### フェーズ別の可視性メモ

| フィールド | DEAL〜BUILD | WALK | ROUND_END |
|------------|-------------|------|-----------|
| `you.wall` | BUILD中は34枚を返す | null（もう使わない） | null |
| `you.hand` / `you.pool` | submit 後に確定値 | 実牌 | 実牌。流局時は残候補も開示 |
| `opponent` の牌中身 | 枚数のみ | 河のみ公開、手牌/候補は枚数 | 和了時は勝者手牌。**流局時は相手手牌・残候補を全公開** |
| `sideWall` | DORA_SELECT 中、先行にのみ**上牌34枚**を返す | null | null |

#### ROUND_END の `roundResult` 例

```json
{
  "roundResult": {
    "result": "RON",
    "winnerSeat": 1,
    "loserSeat": 0,
    "winningTile": { "id": 55, "kind": "ton" },
    "hand": [{ "id": 1, "kind": "m1" }],
    "yaku": ["RIICHI", "YAKUHAI_TON", "DORA"],
    "han": 4,
    "limit": "MANGAN",
    "yakumanCount": 0,
    "pointsAwarded": 1,
    "scoreDelta": [0, 1],
    "acks": [true, false]
  }
}
```

流局時:

```json
{
  "roundResult": {
    "result": "DRAW",
    "winnerSeat": null,
    "pointsAwarded": 0,
    "scoreDelta": [0, 0],
    "acks": [false, false]
  }
}
```

### マスク規則

| 情報 | 自分向け | 相手から見える自分 |
|------|----------|-------------------|
| wall 34枚 | BUILD 中のみ実牌 | `wallCount` のみ |
| hand / pool | 実牌 | 枚数のみ（河は公開） |
| sideWall | DORA_SELECT かつ先行のみ（上牌34） | `sideWallCount` |
| legalTileIds | 手番の自分のみ | 送らない／空 |
| 和了手牌 | ROUND_END で両者に公開 | 同左 |

### `event`（演出・ワンショット）

```json
{
  "type": "event",
  "seq": 18,
  "payload": {
    "kind": "RON",
    "seat": 1,
    "phase": "WALK",
    "tile": { "id": 55, "kind": "ton" },
    "scores": [1, 0]
  }
}
```

#### `kind`

| kind | 用途 |
|------|------|
| `PHASE` | フェーズ変化フック（任意） |
| `DORA` | ドラ確定 |
| `HAND_LOCKED` | 片方/両方の手組み確定 |
| `DISCARD` | 打牌（state と併用可） |
| `RON` | 和了 |
| `DRAW` | 流局 |
| `SCORE` | 得点加算（和了者のみ） |
| `OPPONENT_LEFT` | 切断 |
| `GAME_OVER` | マッチ決着 |

### `error`（個別）

```json
{
  "type": "error",
  "payload": {
    "code": "ILLEGAL_ACTION",
    "message": "tile not in pool",
    "requestId": "optional-uuid"
  }
}
```

| code | 意味 |
|------|------|
| `ROOM_FULL` | 定員超過 |
| `NOT_YOUR_TURN` | 手番違い |
| `ILLEGAL_ACTION` | フェーズ不一致・ルール違反 |
| `INVALID_PAYLOAD` | JSON / tileId 不正 |
| `GAME_OVER` | 終了後の操作 |

### `pong`

```json
{ "type": "pong", "payload": {} }
```

## シーケンス例（1局冒頭）

```
ClientA/B          Server
  |-- ready ------->|
  |-- ready ------->|
  |<---- state LOBBY
  |<---- state DEAL → DORA_SELECT
  |-- select_dora ->|  (先行)
  |<---- state BUILD (wall 付き)
  |-- submit_hand ->|
  |<---- state (handSubmitted)
  |-- submit_hand ->|  (相手)
  |<---- state WALK
  |-- discard_tile ->|
  |<---- state / event DISCARD
  |   … ロンなら event RON → ROUND_END
```
