package game

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTotalRounds     = 4
	DefaultBuildTimeoutSec = 180
	BuildTimeout           = time.Duration(DefaultBuildTimeoutSec) * time.Second
	// BuildTimerEnforced: true のとき BUILD 期限切れで未提出手牌を強制ロックする
	BuildTimerEnforced = true
	MaxTurns           = 17
	WallSize           = 34
	HandSize           = 13
	PoolSize           = 21
)

// MatchConfig is applied when creating a game / rematch.
type MatchConfig struct {
	TotalRounds     int
	BuildTimeout    time.Duration // legacy; used if BuildTimeoutSec==0 && !InfiniteBuild
	BuildTimeoutSec int           // >0 overrides; ignored if InfiniteBuild
	InfiniteBuild   bool          // true → 制限なし（両者OK）
	WallSorted      bool          // 山札を理牌して表示
}

// Phase is the match phase.
type Phase string

const (
	PhaseLobby      Phase = "LOBBY"
	PhaseDeal       Phase = "DEAL"
	PhaseDoraSelect Phase = "DORA_SELECT"
	PhaseBuild      Phase = "BUILD"
	PhaseWalk       Phase = "WALK"
	PhaseRoundEnd   Phase = "ROUND_END"
	PhaseGameOver   Phase = "GAME_OVER"
)

// Seat is 0 or 1.
type Seat int

// Player holds per-seat match state. Score is cumulative mahjong points (加点のみ).
type Player struct {
	ID             string
	Name           string
	Seat           Seat
	Score          int // 累計点（満貫ロン子=8000 など）
	Ready          bool
	Rematch        bool
	Connected      bool // false while awaiting reconnect (grace)
	Wall           []Tile
	Hand           []Tile
	Pool           []Tile
	River          []Tile
	HandSubmitted  bool
	AgariAbandoned bool // 4翻未満ロン牌を受けた後、全待ちあがり放棄
	TurnCount      int
}

// RoundResult is published in ROUND_END.
type RoundResult struct {
	Result           string
	WinnerSeat       *Seat
	LoserSeat        *Seat
	WinningTile      *Tile
	Hand             []Tile
	Waits            []Kind // 和了者の待ち
	Yaku             []string
	YakuItems        []YakuItem
	Han              int
	Limit            Limit
	YakumanCount     int
	PointsAwarded    float64
	Points           int // 加点合計
	HandPoints       int // 役による加点
	ScoreDelta       [2]float64
	Ack              [2]bool
}

// Event is a one-shot notification for clients.
type Event struct {
	Kind  string
	Seat  *Seat
	Tile  *Tile
	Phase Phase
	Extra map[string]any
}

// Game is the authoritative match state.
// Concurrency: Room serializes all access via Room.mu (including BUILD timeout hook).
type Game struct {
	Phase         Phase
	Seq           int
	Round         int
	TotalRounds   int
	LeadSeat      Seat
	ToAct         Seat
	WinnerSeat    *Seat
	SideStacks    [][2]Tile // [0]=表候補(上), [1]=下牌（裏ドラ無しのため不使用）
	DoraIndicator *Tile
	DoraKinds     []Kind
	Players       [2]*Player
	Result        *RoundResult
	rng             *rand.Rand
	BuildTimeout    time.Duration // 0 = 無限
	BuildTimeoutSec int           // 0 = 無限（表示・設定用）
	WallSorted      bool
	BuildEndsAt     time.Time // BUILD 残り時間表示用（ゼロなら無制限/未設定）

	buildTimer *time.Timer
	onUpdate   func()
}

// NewGame creates a lobby-ready game with default match settings.
func NewGame() *Game {
	return NewGameWithConfig(MatchConfig{})
}

// NewGameWithConfig creates a game using the given match settings.
func NewGameWithConfig(cfg MatchConfig) *Game {
	if cfg.TotalRounds <= 0 {
		cfg.TotalRounds = DefaultTotalRounds
	}
	sec := DefaultBuildTimeoutSec
	if cfg.InfiniteBuild {
		sec = 0
	} else if cfg.BuildTimeoutSec > 0 {
		sec = cfg.BuildTimeoutSec
	} else if cfg.BuildTimeout > 0 {
		sec = int(cfg.BuildTimeout / time.Second)
	}
	var dur time.Duration
	if sec > 0 {
		dur = time.Duration(sec) * time.Second
	}
	g := &Game{
		Phase:           PhaseLobby,
		TotalRounds:     cfg.TotalRounds,
		BuildTimeout:    dur,
		BuildTimeoutSec: sec,
		WallSorted:      cfg.WallSorted,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	g.Players[0] = &Player{Seat: 0}
	g.Players[1] = &Player{Seat: 1}
	return g
}

// SetUpdateHook registers a callback for BUILD timeout (Room must lock before reading Game).
func (g *Game) SetUpdateHook(fn func()) {
	g.onUpdate = fn
}

// BumpSeq increments and returns the broadcast sequence.
func (g *Game) BumpSeq() int {
	g.Seq++
	return g.Seq
}

var (
	ErrNotYourTurn    = errors.New("not your turn")
	ErrIllegalAction  = errors.New("illegal action")
	ErrInvalidPayload = errors.New("invalid payload")
	ErrGameOver       = errors.New("game over")
)

// JoinSeat assigns a player to the first free seat.
func (g *Game) JoinSeat(id, name string) (Seat, error) {
	if g.Phase != PhaseLobby {
		return 0, ErrIllegalAction
	}
	for i := 0; i < 2; i++ {
		if g.Players[i].ID == "" {
			g.Players[i].ID = id
			g.Players[i].Name = name
			g.Players[i].Connected = true
			return Seat(i), nil
		}
	}
	return 0, fmt.Errorf("room full")
}

// SeatByPlayerID finds a seat still reserved for playerID (including disconnected).
func (g *Game) SeatByPlayerID(id string) (Seat, bool) {
	if id == "" {
		return 0, false
	}
	for i := 0; i < 2; i++ {
		if g.Players[i].ID == id {
			return Seat(i), true
		}
	}
	return 0, false
}

// ReclaimSeat reattaches a disconnected player to their seat (any phase).
func (g *Game) ReclaimSeat(seat Seat, id, name string) error {
	p := g.Players[seat]
	if p.ID != id {
		return ErrIllegalAction
	}
	if name != "" {
		p.Name = name
	}
	p.Connected = true
	return nil
}

// MarkDisconnected marks a seat offline without clearing match state.
func (g *Game) MarkDisconnected(seat Seat) {
	if g.Players[seat].ID == "" {
		return
	}
	g.Players[seat].Connected = false
}

// LeaveSeat clears a seat (lobby) or ends match if in play.
func (g *Game) LeaveSeat(seat Seat) []Event {
	p := g.Players[seat]
	if g.Phase == PhaseLobby {
		p.ID, p.Name, p.Ready, p.Connected = "", "", false, false
		return nil
	}
	other := Seat(1 - seat)
	g.stopBuildTimer()
	g.Phase = PhaseGameOver
	g.WinnerSeat = &other
	s := other
	p.Connected = false
	return []Event{
		{Kind: "OPPONENT_LEFT", Seat: &s, Phase: PhaseGameOver},
		{Kind: "GAME_OVER", Seat: &s, Phase: PhaseGameOver},
	}
}

// ApplyReady toggles ready in LOBBY; starts match when both ready.
// Prefer StartMatchFromLobby for challenge-accept flow (no ready UI).
func (g *Game) ApplyReady(seat Seat, ready bool) ([]Event, error) {
	if g.Phase != PhaseLobby {
		return nil, ErrIllegalAction
	}
	if g.Players[seat].ID == "" {
		return nil, ErrIllegalAction
	}
	g.Players[seat].Ready = ready
	if g.Players[0].Ready && g.Players[1].Ready && g.Players[0].ID != "" && g.Players[1].ID != "" {
		return g.startMatch(), nil
	}
	return nil, nil
}

// StartMatchFromLobby begins the match immediately when both seats are filled (no ready gate).
func (g *Game) StartMatchFromLobby() ([]Event, error) {
	if g.Phase != PhaseLobby {
		return nil, ErrIllegalAction
	}
	if g.Players[0].ID == "" || g.Players[1].ID == "" {
		return nil, ErrIllegalAction
	}
	g.Players[0].Ready = true
	g.Players[1].Ready = true
	return g.startMatch(), nil
}

func (g *Game) startMatch() []Event {
	g.Round = 1
	g.LeadSeat = Seat(g.rng.Intn(2))
	g.Players[0].Score = 0
	g.Players[1].Score = 0
	g.WinnerSeat = nil
	return g.deal()
}

func (g *Game) deal() []Event {
	g.stopBuildTimer()
	tiles := NewShuffledTiles(g.rng)
	w0, w1, side := DealSplit(tiles)
	g.Players[0].Wall = w0
	g.Players[1].Wall = w1
	g.Players[0].Hand, g.Players[0].Pool, g.Players[0].River = nil, nil, nil
	g.Players[1].Hand, g.Players[1].Pool, g.Players[1].River = nil, nil, nil
	g.Players[0].HandSubmitted, g.Players[1].HandSubmitted = false, false
	g.Players[0].AgariAbandoned, g.Players[1].AgariAbandoned = false, false
	g.Players[0].TurnCount, g.Players[1].TurnCount = 0, 0
	g.SideStacks = PairSideWall(side)
	g.DoraIndicator = nil
	g.DoraKinds = nil
	g.Result = nil
	g.ToAct = g.LeadSeat
	// ドラ表示牌は脇山からランダム自動選定 → 即 BUILD
	return g.autoPickDora()
}

// autoPickDora selects a random side-wall stack top as the dora indicator and starts BUILD.
func (g *Game) autoPickDora() []Event {
	if len(g.SideStacks) == 0 {
		g.Phase = PhaseBuild
		g.armBuildDeadline()
		return []Event{{Kind: "PHASE", Phase: PhaseBuild}}
	}
	idx := g.rng.Intn(len(g.SideStacks))
	top := g.SideStacks[idx][0]
	g.SideStacks = nil
	g.DoraIndicator = &top
	g.DoraKinds = []Kind{NextKind(top.Kind)}
	g.Phase = PhaseBuild
	g.armBuildDeadline()
	return []Event{
		{Kind: "DORA", Phase: PhaseBuild, Tile: &top},
		{Kind: "PHASE", Phase: PhaseBuild},
	}
}

// ApplySelectDora is kept for protocol compatibility; preferential path is autoPickDora.
func (g *Game) ApplySelectDora(seat Seat, tileID int) ([]Event, error) {
	if g.Phase != PhaseDoraSelect {
		return nil, ErrIllegalAction
	}
	if seat != g.LeadSeat || seat != g.ToAct {
		return nil, ErrNotYourTurn
	}
	idx := -1
	for i, st := range g.SideStacks {
		if st[0].ID == tileID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, ErrInvalidPayload
	}
	top := g.SideStacks[idx][0]
	g.SideStacks = nil
	g.DoraIndicator = &top
	g.DoraKinds = []Kind{NextKind(top.Kind)}
	g.Phase = PhaseBuild
	g.armBuildDeadline()
	return []Event{
		{Kind: "DORA", Phase: PhaseBuild, Tile: &top},
		{Kind: "PHASE", Phase: PhaseBuild},
	}, nil
}

func (g *Game) armBuildDeadline() {
	g.stopBuildTimer()
	if g.BuildTimeoutSec <= 0 {
		g.BuildEndsAt = time.Time{}
		return
	}
	g.BuildEndsAt = time.Now().Add(time.Duration(g.BuildTimeoutSec) * time.Second)
	g.startBuildTimer()
}

// BuildRemainingSec returns ceiling seconds left in BUILD (0 if none / expired / infinite).
func (g *Game) BuildRemainingSec() int {
	if g.Phase != PhaseBuild || g.BuildTimeoutSec <= 0 || g.BuildEndsAt.IsZero() {
		return 0
	}
	d := time.Until(g.BuildEndsAt)
	if d <= 0 {
		return 0
	}
	sec := int(d.Seconds())
	if d > time.Duration(sec)*time.Second {
		sec++
	}
	return sec
}

// BuildTimeoutSecValue returns configured BUILD limit (0 = infinite).
func (g *Game) BuildTimeoutSecValue() int {
	return g.BuildTimeoutSec
}

// ApplySetOptions updates lobby match options (timeout / rounds / wall sort). Clears ready flags.
// buildTimeoutSec: 0 = infinite; allowed finite values 120/180/300.
// totalRounds: 4 | 6 | 8 | 10.
func (g *Game) ApplySetOptions(seat Seat, buildTimeoutSec int, totalRounds int, wallSorted bool) ([]Event, error) {
	if g.Phase != PhaseLobby {
		return nil, ErrIllegalAction
	}
	if g.Players[seat].ID == "" {
		return nil, ErrIllegalAction
	}
	if !AllowedBuildTimeoutSec(buildTimeoutSec) || !AllowedTotalRounds(totalRounds) {
		return nil, ErrInvalidPayload
	}
	g.BuildTimeoutSec = buildTimeoutSec
	if buildTimeoutSec > 0 {
		g.BuildTimeout = time.Duration(buildTimeoutSec) * time.Second
	} else {
		g.BuildTimeout = 0
	}
	g.TotalRounds = totalRounds
	g.WallSorted = wallSorted
	g.Players[0].Ready = false
	g.Players[1].Ready = false
	preset := "inf"
	if buildTimeoutSec > 0 {
		preset = fmt.Sprintf("%d", buildTimeoutSec)
	}
	return []Event{{Kind: "OPTIONS", Phase: PhaseLobby, Extra: map[string]any{
		"timeoutPreset":   preset,
		"buildTimeoutSec": buildTimeoutSec,
		"totalRounds":     totalRounds,
		"wallSorted":      wallSorted,
	}}}, nil
}

// AllowedTotalRounds reports whether n is a lobby-selectable match length.
func AllowedTotalRounds(n int) bool {
	switch n {
	case 4, 6, 8, 10:
		return true
	default:
		return false
	}
}

// AllowedBuildTimeoutSec reports whether sec is a lobby-selectable build timeout (0 = infinite).
func AllowedBuildTimeoutSec(sec int) bool {
	switch sec {
	case 0, 120, 180, 300:
		return true
	default:
		return false
	}
}

// ParseTimeoutPreset maps lobby option strings to seconds (0 = infinite).
func ParseTimeoutPreset(preset string, fallbackSec int) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "inf", "infinite", "none", "0":
		return 0, true
	case "120", "180", "300":
		n, _ := strconv.Atoi(preset)
		return n, true
	case "":
		switch fallbackSec {
		case 0, 120, 180, 300:
			return fallbackSec, true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func (g *Game) startBuildTimer() {
	g.stopBuildTimer()
	if !BuildTimerEnforced || g.BuildTimeoutSec <= 0 || g.BuildEndsAt.IsZero() {
		return
	}
	delay := time.Until(g.BuildEndsAt)
	if delay < 0 {
		delay = 0
	}
	g.buildTimer = time.AfterFunc(delay, func() {
		if g.onUpdate != nil {
			g.onUpdate()
		}
	})
}

func (g *Game) stopBuildTimer() {
	if g.buildTimer != nil {
		g.buildTimer.Stop()
		g.buildTimer = nil
	}
}

// ForceBuildTimeout forces unsubmitted hands when BUILD timer is enforced.
func (g *Game) ForceBuildTimeout() []Event {
	if !BuildTimerEnforced {
		return nil
	}
	if g.Phase != PhaseBuild {
		return nil
	}
	if g.BuildTimeoutSec <= 0 || g.BuildEndsAt.IsZero() {
		return nil
	}
	if time.Now().Before(g.BuildEndsAt) {
		return nil
	}
	return g.forceBuild()
}

func (g *Game) forceBuild() []Event {
	var events []Event
	for i := 0; i < 2; i++ {
		p := g.Players[i]
		if p.HandSubmitted {
			continue
		}
		if len(p.Wall) < HandSize {
			continue
		}
		p.Hand = append([]Tile(nil), p.Wall[:HandSize]...)
		p.Pool = append([]Tile(nil), p.Wall[HandSize:]...)
		p.HandSubmitted = true
		s := Seat(i)
		events = append(events, Event{Kind: "HAND_LOCKED", Seat: &s, Phase: PhaseBuild})
	}
	if g.Players[0].HandSubmitted && g.Players[1].HandSubmitted {
		g.stopBuildTimer()
		g.Phase = PhaseWalk
		g.ToAct = g.LeadSeat
		events = append(events, Event{Kind: "PHASE", Phase: PhaseWalk})
	}
	return events
}

// PreviewWaits returns tenpai waits for a candidate 13-tile hand from the wall (BUILD only).
// Empty slice means not tenpai (or invalid); error on illegal phase / bad ids.
func (g *Game) PreviewWaits(seat Seat, tileIDs []int) ([]Kind, error) {
	if g.Phase != PhaseBuild {
		return nil, ErrIllegalAction
	}
	p := g.Players[seat]
	if p.ID == "" || p.HandSubmitted {
		return nil, ErrIllegalAction
	}
	if len(tileIDs) != HandSize {
		return nil, ErrInvalidPayload
	}
	hand, _, ok := PickTilesByID(p.Wall, tileIDs)
	if !ok {
		return nil, ErrInvalidPayload
	}
	return AllWaits(hand), nil
}

// ApplySubmitHand locks a 13-tile hand from the player's wall.
func (g *Game) ApplySubmitHand(seat Seat, tileIDs []int) ([]Event, error) {
	if g.Phase != PhaseBuild {
		return nil, ErrIllegalAction
	}
	p := g.Players[seat]
	if p.HandSubmitted {
		return nil, ErrIllegalAction
	}
	if len(tileIDs) != HandSize {
		return nil, ErrInvalidPayload
	}
	hand, remain, ok := PickTilesByID(p.Wall, tileIDs)
	if !ok || len(remain) != PoolSize {
		return nil, ErrInvalidPayload
	}
	p.Hand = hand
	p.Pool = remain
	p.HandSubmitted = true
	s := seat
	events := []Event{{Kind: "HAND_LOCKED", Seat: &s, Phase: PhaseBuild}}
	if g.Players[0].HandSubmitted && g.Players[1].HandSubmitted {
		g.stopBuildTimer()
		g.Phase = PhaseWalk
		g.ToAct = g.LeadSeat
		events = append(events, Event{Kind: "PHASE", Phase: PhaseWalk})
	}
	return events, nil
}

// ApplyDiscard handles WALK discard and automatic ron.
func (g *Game) ApplyDiscard(seat Seat, tileID int) ([]Event, error) {
	if g.Phase != PhaseWalk {
		return nil, ErrIllegalAction
	}
	if seat != g.ToAct {
		return nil, ErrNotYourTurn
	}
	p := g.Players[seat]
	if p.TurnCount >= MaxTurns {
		return nil, ErrIllegalAction
	}
	pool, t, ok := RemoveTileID(p.Pool, tileID)
	if !ok {
		return nil, ErrInvalidPayload
	}
	p.Pool = pool
	p.River = append(p.River, t)
	p.TurnCount++
	s := seat
	events := []Event{{Kind: "DISCARD", Seat: &s, Tile: &t, Phase: PhaseWalk}}

	opp := Seat(1 - seat)
	op := g.Players[opp]
	// Auto-ron (no miss): 4+ han → immediate ron; under 4 han → forfeit ALL waits.
	ev, under := EvaluateRon(RonContext{
		Hand:           op.Hand,
		WinningTile:    t,
		River:          op.River,
		DoraKinds:      g.DoraKinds,
		AgariAbandoned: op.AgariAbandoned,
		IsLead:         false, // 親子なし（自風・点数とも子扱い）
		DiscarderTurn:  p.TurnCount,
		WinnerTurn:     op.TurnCount,
		// 河底: 後攻の第17打
		IsLastDiscard: p.TurnCount == MaxTurns && seat != g.LeadSeat,
	})
	if under {
		op.AgariAbandoned = true
	} else if ev.Winning {
		return append(events, g.finishRon(opp, seat, t, ev)...), nil
	}

	if g.Players[0].TurnCount >= MaxTurns && g.Players[1].TurnCount >= MaxTurns {
		return append(events, g.finishDraw()...), nil
	}
	if g.Players[opp].TurnCount < MaxTurns {
		g.ToAct = opp
	} else if p.TurnCount < MaxTurns {
		g.ToAct = seat
	}
	return events, nil
}

// FinishDrawForTest ends the current walk as a draw (integration tests).
func (g *Game) FinishDrawForTest() []Event {
	if g.Phase != PhaseWalk {
		return nil
	}
	return g.finishDraw()
}

func (g *Game) finishRon(winner, loser Seat, tile Tile, ev Evaluation) []Event {
	g.stopBuildTimer()
	wp := g.Players[winner]
	// 常に子の点数表で算出（親子差なし）
	handPts := RonPoints(ev.Limit, ev.YakumanCount)
	if handPts <= 0 && ev.Han >= 4 {
		handPts = RonPoints(limitFromHan(ev.Han), 0)
	}
	total := handPts
	wp.Score += total
	items := ev.YakuItems
	if len(items) == 0 {
		items = ExpandYakuItems(ev.Yaku, ev.Han)
	}
	w, l := winner, loser
	g.Result = &RoundResult{
		Result:        "RON",
		WinnerSeat:    &w,
		LoserSeat:     &l,
		WinningTile:   &tile,
		Hand:          append([]Tile(nil), wp.Hand...),
		Waits:         AllWaits(wp.Hand),
		Yaku:          ev.Yaku,
		YakuItems:     items,
		Han:           ev.Han,
		Limit:         ev.Limit,
		YakumanCount:  ev.YakumanCount,
		PointsAwarded: float64(total),
		Points:        total,
		HandPoints:    handPts,
	}
	g.Result.ScoreDelta[winner] = float64(total)
	g.Phase = PhaseRoundEnd
	return []Event{
		{Kind: "RON", Seat: &w, Tile: &tile, Phase: PhaseRoundEnd},
		{Kind: "SCORE", Seat: &w, Phase: PhaseRoundEnd},
		{Kind: "PHASE", Phase: PhaseRoundEnd},
	}
}

func (g *Game) finishDraw() []Event {
	g.stopBuildTimer()
	g.Result = &RoundResult{Result: "DRAW"}
	g.Phase = PhaseRoundEnd
	return []Event{{Kind: "DRAW", Phase: PhaseRoundEnd}, {Kind: "PHASE", Phase: PhaseRoundEnd}}
}

// ApplyAckRound acknowledges ROUND_END; advances or ends match when both acked.
func (g *Game) ApplyAckRound(seat Seat) ([]Event, error) {
	if g.Phase != PhaseRoundEnd || g.Result == nil {
		return nil, ErrIllegalAction
	}
	g.Result.Ack[seat] = true
	if !g.Result.Ack[0] || !g.Result.Ack[1] {
		return nil, nil
	}
	if done, events := g.tryFinishAfterRound(); done {
		return events, nil
	}
	// 連荘なし: 毎局先行交代・局数+1
	g.LeadSeat = Seat(1 - g.LeadSeat)
	g.Round++
	return g.deal(), nil
}

// tryFinishAfterRound ends the match after the final scheduled round.
func (g *Game) tryFinishAfterRound() (bool, []Event) {
	if g.Round < g.TotalRounds {
		return false, nil
	}
	return true, g.endMatch()
}

func (g *Game) endMatch() []Event {
	g.stopBuildTimer()
	g.Phase = PhaseGameOver
	g.WinnerSeat = nil
	if g.Players[0].Score > g.Players[1].Score {
		s := Seat(0)
		g.WinnerSeat = &s
	} else if g.Players[1].Score > g.Players[0].Score {
		s := Seat(1)
		g.WinnerSeat = &s
	}
	return []Event{{Kind: "GAME_OVER", Seat: g.WinnerSeat, Phase: PhaseGameOver}}
}

// SideTops returns selectable dora indicator candidates (stack tops).
func (g *Game) SideTops() []Tile {
	out := make([]Tile, 0, len(g.SideStacks))
	for _, st := range g.SideStacks {
		out = append(out, st[0])
	}
	return out
}

// ApplyRematch records rematch intent; restarts when both agree.
func (g *Game) ApplyRematch(seat Seat) ([]Event, error) {
	if g.Phase != PhaseGameOver {
		return nil, ErrIllegalAction
	}
	g.Players[seat].Rematch = true
	if g.Players[0].Rematch && g.Players[1].Rematch {
		g.Players[0].Rematch, g.Players[1].Rematch = false, false
		g.Players[0].Ready, g.Players[1].Ready = true, true
		return g.startMatch(), nil
	}
	return nil, nil
}

// ScoresDisplay returns cumulative scores for protocol.
func (g *Game) ScoresDisplay() [2]float64 {
	return [2]float64{
		float64(g.Players[0].Score),
		float64(g.Players[1].Score),
	}
}
