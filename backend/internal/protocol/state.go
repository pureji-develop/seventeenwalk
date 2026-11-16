package protocol

import (
	"seventeenwalk/internal/game"
)

// TileView is JSON tile.
type TileView struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
}

func tileView(t game.Tile) TileView {
	return TileView{ID: t.ID, Kind: string(t.Kind)}
}

func tileViews(tiles []game.Tile) []TileView {
	if tiles == nil {
		return nil
	}
	out := make([]TileView, len(tiles))
	for i, t := range tiles {
		out[i] = tileView(t)
	}
	return out
}

func tileViewPtr(t *game.Tile) *TileView {
	if t == nil {
		return nil
	}
	v := tileView(*t)
	return &v
}

// YouView is masked self view.
type YouView struct {
	Seat           int        `json:"seat"`
	Name           string     `json:"name"`
	Ready          bool       `json:"ready"`
	Rematch        bool       `json:"rematch"`
	Connected      bool       `json:"connected"`
	Wall           []TileView `json:"wall"`
	Hand           []TileView `json:"hand"`
	Pool           []TileView `json:"pool"`
	PoolCount      int        `json:"poolCount"`
	Waits          []string   `json:"waits,omitempty"` // BUILD/WALK: テンパイ待ち
	HandSubmitted  bool       `json:"handSubmitted"`
	AgariAbandoned bool       `json:"agariAbandoned"`
	River          []TileView `json:"river"`
	TurnCount      int        `json:"turnCount"`
}

// OpponentView is masked opponent view.
type OpponentView struct {
	Seat           int        `json:"seat"`
	Name           string     `json:"name"`
	Ready          bool       `json:"ready"`
	Rematch        bool       `json:"rematch"`
	Connected      bool       `json:"connected"`
	WallCount      int        `json:"wallCount"`
	HandCount      int        `json:"handCount"`
	PoolCount      int        `json:"poolCount"`
	Hand           []TileView `json:"hand,omitempty"` // 和了時・流局時など公開時のみ
	Pool           []TileView `json:"pool,omitempty"` // 流局時など残牌公開時のみ
	HandSubmitted  bool       `json:"handSubmitted"`
	AgariAbandoned bool       `json:"agariAbandoned"`
	River          []TileView `json:"river"`
	TurnCount      int        `json:"turnCount"`
}

// WalkView holds turn helpers.
type WalkView struct {
	LegalTileIDs []int `json:"legalTileIds"`
	MaxTurns     int   `json:"maxTurns"`
}

// RoundResultView is ROUND_END payload fragment.
type RoundResultView struct {
	Result           string     `json:"result"`
	WinnerSeat       *int       `json:"winnerSeat"`
	LoserSeat        *int       `json:"loserSeat"`
	WinningTile      *TileView  `json:"winningTile"`
	Hand             []TileView `json:"hand"`
	Waits            []string   `json:"waits"`
	Yaku             []string   `json:"yaku"`
	YakuDetails      []YakuDetail `json:"yakuDetails"`
	Han              int        `json:"han"`
	Limit            string     `json:"limit"`
	YakumanCount     int        `json:"yakumanCount"`
	PointsAwarded    float64    `json:"pointsAwarded"`
	HandPoints       int        `json:"handPoints"`
	ScoreDelta       [2]float64 `json:"scoreDelta"`
	DoraIndicator    *TileView  `json:"doraIndicator"`
	Dora             []string   `json:"dora"`
	Acks             [2]bool    `json:"acks"`
}

// YakuDetail is one yaku line for the ron panel.
type YakuDetail struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Han  int    `json:"han"`
}

// StatePayload is the masked full snapshot.
type StatePayload struct {
	RoomID           string           `json:"roomId"`
	Phase            string           `json:"phase"`
	Round            int              `json:"round"`
	TotalRounds      int              `json:"totalRounds"`
	LeadSeat         int              `json:"leadSeat"`
	ToAct            int              `json:"toAct"`
	Scores           [2]float64       `json:"scores"`
	DoraIndicator    *TileView        `json:"doraIndicator"`
	Dora             []string         `json:"dora"`
	You              YouView          `json:"you"`
	Opponent         OpponentView     `json:"opponent"`
	SideWallCount    int              `json:"sideWallCount"`
	SideWall         []TileView       `json:"sideWall"`
	Walk             *WalkView        `json:"walk"`
	RoundResult      *RoundResultView `json:"roundResult"`
	WinnerSeat       *int             `json:"winnerSeat"`
	BuildTimeoutSec   int              `json:"buildTimeoutSec"`
	BuildEndsAtMs     int64            `json:"buildEndsAtMs"`
	BuildRemainingSec int              `json:"buildRemainingSec"`
	BuildTimerEnabled bool             `json:"buildTimerEnabled"`
	WallSorted        bool             `json:"wallSorted"`
	BuildInfinite     bool             `json:"buildInfinite"`
	// Challenge is set while a guest awaits host approval in LOBBY.
	Challenge *ChallengeView `json:"challenge,omitempty"`
}

// ChallengeView is a pending join-request in LOBBY.
type ChallengeView struct {
	FromName    string `json:"fromName"`
	FromSeat    int    `json:"fromSeat"`
	ExpiresAtMs int64  `json:"expiresAtMs"`
	Role        string `json:"role"` // "host" | "challenger"
}

// BuildState builds a viewer-specific snapshot. Caller must hold game lock or guarantee immutability.
func BuildState(g *game.Game, roomID string, viewer game.Seat) StatePayload {
	you := g.Players[viewer]
	oppSeat := game.Seat(1 - viewer)
	opp := g.Players[oppSeat]
	scores := g.ScoresDisplay()

	yv := YouView{
		Seat:           int(you.Seat),
		Name:           you.Name,
		Ready:          you.Ready,
		Rematch:        you.Rematch,
		Connected:      you.Connected,
		HandSubmitted:  you.HandSubmitted,
		AgariAbandoned: you.AgariAbandoned,
		River:          tileViews(you.River),
		TurnCount:      you.TurnCount,
		PoolCount:      len(you.Pool),
	}
	ov := OpponentView{
		Seat:           int(opp.Seat),
		Name:           opp.Name,
		Ready:          opp.Ready,
		Rematch:        opp.Rematch,
		Connected:      opp.Connected,
		HandSubmitted:  opp.HandSubmitted,
		AgariAbandoned: opp.AgariAbandoned,
		River:          tileViews(opp.River),
		TurnCount:      opp.TurnCount,
		WallCount:      len(opp.Wall),
		HandCount:      len(opp.Hand),
		PoolCount:      len(opp.Pool),
	}

	switch g.Phase {
	case game.PhaseBuild, game.PhaseDoraSelect, game.PhaseDeal:
		yv.Wall = tileViews(you.Wall)
		// OK 済みなら確定手牌も自分に見せる（相手待ち中の確認用）
		if you.HandSubmitted {
			yv.Hand = tileViews(you.Hand)
			yv.Waits = kindsToStrings(game.AllWaits(you.Hand))
		}
	case game.PhaseWalk, game.PhaseRoundEnd:
		yv.Hand = tileViews(you.Hand)
		yv.Pool = tileViews(you.Pool)
		yv.Waits = kindsToStrings(game.AllWaits(you.Hand))
		if g.Phase == game.PhaseRoundEnd && g.Result != nil {
			switch g.Result.Result {
			case "DRAW", "RON":
				// 流局・和了とも盤面確認用に相手手牌・残候補を公開
				ov.Hand = tileViews(opp.Hand)
				ov.HandCount = len(opp.Hand)
				ov.Pool = tileViews(opp.Pool)
				ov.PoolCount = len(opp.Pool)
			}
		}
	}

	var sideWall []TileView
	if g.Phase == game.PhaseDoraSelect && viewer == g.LeadSeat {
		sideWall = tileViews(g.SideTops())
	}

	var walk *WalkView
	if g.Phase == game.PhaseWalk {
		legal := []int{}
		if viewer == g.ToAct {
			for _, t := range you.Pool {
				legal = append(legal, t.ID)
			}
		}
		walk = &WalkView{LegalTileIDs: legal, MaxTurns: game.MaxTurns}
	}

	var rr *RoundResultView
	if g.Result != nil {
		rr = &RoundResultView{
			Result:        g.Result.Result,
			Yaku:          g.Result.Yaku,
			YakuDetails:   yakuDetailsFromItems(g.Result.YakuItems),
			Han:           g.Result.Han,
			Limit:         string(g.Result.Limit),
			YakumanCount:  g.Result.YakumanCount,
			PointsAwarded: g.Result.PointsAwarded,
			HandPoints:    g.Result.HandPoints,
			ScoreDelta:    g.Result.ScoreDelta,
			Acks:          g.Result.Ack,
			Hand:          tileViews(g.Result.Hand),
			WinningTile:   tileViewPtr(g.Result.WinningTile),
			Waits:         kindsToStrings(g.Result.Waits),
		}
		if g.Result.Result == "RON" {
			rr.DoraIndicator = tileViewPtr(g.DoraIndicator)
			rr.Dora = kindsToStrings(g.DoraKinds)
		}
		if g.Result.WinnerSeat != nil {
			w := int(*g.Result.WinnerSeat)
			rr.WinnerSeat = &w
		}
		if g.Result.LoserSeat != nil {
			l := int(*g.Result.LoserSeat)
			rr.LoserSeat = &l
		}
	}

	dora := kindsToStrings(g.DoraKinds)

	var winner *int
	if g.WinnerSeat != nil {
		w := int(*g.WinnerSeat)
		winner = &w
	}

	return StatePayload{
		RoomID:            roomID,
		Phase:             string(g.Phase),
		Round:             g.Round,
		TotalRounds:       g.TotalRounds,
		LeadSeat:          int(g.LeadSeat),
		ToAct:             int(g.ToAct),
		Scores:            scores,
		DoraIndicator:     tileViewPtr(g.DoraIndicator),
		Dora:              dora,
		You:               yv,
		Opponent:          ov,
		SideWallCount:     len(g.SideStacks),
		SideWall:          sideWall,
		Walk:              walk,
		RoundResult:       rr,
		WinnerSeat:        winner,
		BuildTimeoutSec:   g.BuildTimeoutSecValue(),
		BuildEndsAtMs:     buildEndsAtMs(g),
		BuildRemainingSec: g.BuildRemainingSec(),
		BuildTimerEnabled: game.BuildTimerEnforced,
		WallSorted:        g.WallSorted,
		BuildInfinite:     g.BuildTimeoutSecValue() <= 0,
	}
}

func buildEndsAtMs(g *game.Game) int64 {
	if g.Phase != game.PhaseBuild || g.BuildEndsAt.IsZero() {
		return 0
	}
	return g.BuildEndsAt.UnixMilli()
}

func kindsToStrings(kinds []game.Kind) []string {
	return KindsToStrings(kinds)
}

func yakuDetailsFromItems(items []game.YakuItem) []YakuDetail {
	if len(items) == 0 {
		return nil
	}
	out := make([]YakuDetail, len(items))
	for i, it := range items {
		out[i] = YakuDetail{Code: it.Code, Han: it.Han}
	}
	return out
}

// KindsToStrings converts tile kinds to wire strings.
// Always returns a non-nil slice so JSON encodes as [] instead of null.
func KindsToStrings(kinds []game.Kind) []string {
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, string(k))
	}
	return out
}

// WelcomePayload is sent on connect.
type WelcomePayload struct {
	PlayerID string `json:"playerId"`
	Seat     int    `json:"seat"`
	RoomID   string `json:"roomId"`
}

// ErrorPayload is an error response.
type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

// EventPayload is a one-shot event.
type EventPayload struct {
	Kind   string         `json:"kind"`
	Seat   *int           `json:"seat,omitempty"`
	Phase  string         `json:"phase,omitempty"`
	Tile   *TileView      `json:"tile,omitempty"`
	Scores [2]float64     `json:"scores,omitempty"`
	Extra  map[string]any `json:"extra,omitempty"`
}

// BuildEvent maps game.Event to protocol payload.
func BuildEvent(g *game.Game, ev game.Event) EventPayload {
	p := EventPayload{
		Kind:   ev.Kind,
		Phase:  string(ev.Phase),
		Scores: g.ScoresDisplay(),
		Extra:  ev.Extra,
		Tile:   tileViewPtr(ev.Tile),
	}
	if ev.Seat != nil {
		s := int(*ev.Seat)
		p.Seat = &s
	}
	return p
}
