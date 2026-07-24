package game

import (
	"math/rand"
	"testing"
)

func readyBuildGame(t *testing.T) *Game {
	t.Helper()
	g := NewGameWithConfig(MatchConfig{TotalRounds: 2, BuildTimeoutSec: 180})
	g.rng = rand.New(rand.NewSource(1))
	if _, err := g.JoinSeat("a", "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.JoinSeat("b", "B"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ApplyReady(0, true); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ApplyReady(1, true); err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseBuild {
		t.Fatalf("phase=%s want BUILD", g.Phase)
	}
	if g.DoraIndicator == nil {
		t.Fatal("expected auto dora")
	}
	return g
}

func wallIDs(wall []Tile, n int) []int {
	ids := make([]int, n)
	for i := 0; i < n; i++ {
		ids[i] = wall[i].ID
	}
	return ids
}

func TestSubmitBothHandsEntersWalk(t *testing.T) {
	g := readyBuildGame(t)
	ids0 := wallIDs(g.Players[0].Wall, HandSize)
	ids1 := wallIDs(g.Players[1].Wall, HandSize)

	ev, err := g.ApplySubmitHand(0, ids0)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseBuild || !g.Players[0].HandSubmitted {
		t.Fatal("first submit should stay in BUILD")
	}
	if len(ev) != 1 || ev[0].Kind != "HAND_LOCKED" {
		t.Fatalf("events=%v", eventKinds(ev))
	}

	ev, err = g.ApplySubmitHand(1, ids1)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseWalk {
		t.Fatalf("phase=%s", g.Phase)
	}
	if g.ToAct != g.LeadSeat {
		t.Fatalf("toAct=%d lead=%d", g.ToAct, g.LeadSeat)
	}
	if len(g.Players[0].Hand) != HandSize || len(g.Players[0].Pool) != PoolSize {
		t.Fatalf("seat0 hand=%d pool=%d", len(g.Players[0].Hand), len(g.Players[0].Pool))
	}
	if len(ev) < 2 || ev[1].Kind != "PHASE" {
		t.Fatalf("events=%v", eventKinds(ev))
	}
}

func TestPreviewWaitsTenpaiAndRejects(t *testing.T) {
	g := readyBuildGame(t)
	// Force a known tenpai wall for seat 0.
	g.Players[0].Wall = tilesOf(
		KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5,
		KindS6, KindS6, KindS7, KindS7, KindS8,
		KindM1, KindM4, KindM5, KindM6, KindM7, KindM8, KindM9,
		KindP1, KindP2, KindP3, KindP6, KindP7, KindP8, KindP9,
		KindS1, KindS2, KindS3, KindS4, KindS5, KindS9, KindTon,
	)
	tenpaiIDs := wallIDs(g.Players[0].Wall, HandSize)
	waits, err := g.PreviewWaits(0, tenpaiIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(waits) == 0 {
		t.Fatal("expected tenpai waits")
	}

	if _, err := g.PreviewWaits(0, tenpaiIDs[:12]); err != ErrInvalidPayload {
		t.Fatalf("short hand err=%v", err)
	}
	bad := append([]int{}, tenpaiIDs...)
	bad[0] = 99999
	if _, err := g.PreviewWaits(0, bad); err != ErrInvalidPayload {
		t.Fatalf("missing id err=%v", err)
	}
	g.Phase = PhaseWalk
	if _, err := g.PreviewWaits(0, tenpaiIDs); err != ErrIllegalAction {
		t.Fatalf("wrong phase err=%v", err)
	}
}

func TestApplySetOptionsRoundsAndRejects(t *testing.T) {
	g := NewGame()
	g.JoinSeat("a", "A")
	if _, err := g.ApplySetOptions(0, 180, 10, false); err != nil {
		t.Fatal(err)
	}
	if g.TotalRounds != 10 || g.BuildTimeoutSec != 180 {
		t.Fatalf("rounds=%d sec=%d", g.TotalRounds, g.BuildTimeoutSec)
	}
	if _, err := g.ApplySetOptions(0, 180, 5, false); err != ErrInvalidPayload {
		t.Fatalf("invalid rounds err=%v", err)
	}
	if _, err := g.ApplySetOptions(0, 90, 4, false); err != ErrInvalidPayload {
		t.Fatalf("invalid timeout err=%v", err)
	}
	if !AllowedTotalRounds(10) || AllowedTotalRounds(5) {
		t.Fatal("AllowedTotalRounds")
	}
	if !AllowedBuildTimeoutSec(0) || AllowedBuildTimeoutSec(60) {
		t.Fatal("AllowedBuildTimeoutSec")
	}
}

func TestDrawAckEndsMatchOnLastRound(t *testing.T) {
	g := readyBuildGame(t)
	g.TotalRounds = 1
	_, _ = g.ApplySubmitHand(0, wallIDs(g.Players[0].Wall, HandSize))
	_, _ = g.ApplySubmitHand(1, wallIDs(g.Players[1].Wall, HandSize))
	if g.Phase != PhaseWalk {
		t.Fatal(g.Phase)
	}
	ev := g.FinishDrawForTest()
	if g.Phase != PhaseRoundEnd || g.Result.Result != "DRAW" {
		t.Fatalf("phase=%s result=%v events=%v", g.Phase, g.Result, eventKinds(ev))
	}
	if _, err := g.ApplyAckRound(0); err != nil {
		t.Fatal(err)
	}
	ev, err := g.ApplyAckRound(1)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseGameOver {
		t.Fatalf("phase=%s want GAME_OVER", g.Phase)
	}
	if len(ev) == 0 || ev[0].Kind != "GAME_OVER" {
		t.Fatalf("events=%v", eventKinds(ev))
	}
}

func TestDrawAckAdvancesWithoutRenchan(t *testing.T) {
	g := readyBuildGame(t)
	g.TotalRounds = 2
	lead := g.LeadSeat
	_, _ = g.ApplySubmitHand(0, wallIDs(g.Players[0].Wall, HandSize))
	_, _ = g.ApplySubmitHand(1, wallIDs(g.Players[1].Wall, HandSize))
	_ = g.FinishDrawForTest()
	g.ApplyAckRound(0)
	g.ApplyAckRound(1)
	if g.Phase != PhaseBuild {
		t.Fatalf("phase=%s want BUILD for round 2", g.Phase)
	}
	if g.Round != 2 {
		t.Fatalf("round=%d", g.Round)
	}
	if g.LeadSeat == lead {
		t.Fatal("lead should rotate (no renchan)")
	}
}

func TestRematchRestartsMatch(t *testing.T) {
	g := readyBuildGame(t)
	g.TotalRounds = 1
	_, _ = g.ApplySubmitHand(0, wallIDs(g.Players[0].Wall, HandSize))
	_, _ = g.ApplySubmitHand(1, wallIDs(g.Players[1].Wall, HandSize))
	_ = g.FinishDrawForTest()
	g.ApplyAckRound(0)
	g.ApplyAckRound(1)
	if g.Phase != PhaseGameOver {
		t.Fatal(g.Phase)
	}
	if _, err := g.ApplyRematch(0); err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseGameOver {
		t.Fatal("one rematch should wait")
	}
	ev, err := g.ApplyRematch(1)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseBuild || g.Round != 1 {
		t.Fatalf("phase=%s round=%d", g.Phase, g.Round)
	}
	if len(ev) == 0 {
		t.Fatal("expected deal events")
	}
}

func TestLeaveSeatDuringPlayEndsMatch(t *testing.T) {
	g := readyBuildGame(t)
	ev := g.LeaveSeat(0)
	if g.Phase != PhaseGameOver {
		t.Fatal(g.Phase)
	}
	if g.WinnerSeat == nil || *g.WinnerSeat != 1 {
		t.Fatalf("winner=%v", g.WinnerSeat)
	}
	kinds := eventKinds(ev)
	if len(kinds) < 2 || kinds[0] != "OPPONENT_LEFT" || kinds[1] != "GAME_OVER" {
		t.Fatalf("events=%v", kinds)
	}
}

func TestPickTilesByID(t *testing.T) {
	wall := tilesOf(KindM1, KindM2, KindM3, KindM4)
	picked, remain, ok := PickTilesByID(wall, []int{2, 4})
	if !ok || len(picked) != 2 || len(remain) != 2 {
		t.Fatalf("ok=%v picked=%d remain=%d", ok, len(picked), len(remain))
	}
	if picked[0].Kind != KindM2 || picked[1].Kind != KindM4 {
		t.Fatalf("order %#v", picked)
	}
	if _, _, ok := PickTilesByID(wall, []int{1, 1}); ok {
		t.Fatal("duplicate should fail")
	}
	if _, _, ok := PickTilesByID(wall, []int{99}); ok {
		t.Fatal("missing should fail")
	}
}
