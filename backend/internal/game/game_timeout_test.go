package game

import (
	"math/rand"
	"testing"
	"time"
)

func TestApplySelectDora(t *testing.T) {
	g := NewGame()
	g.JoinSeat("a", "A")
	g.JoinSeat("b", "B")
	g.LeadSeat = 0
	g.ToAct = 0
	g.Phase = PhaseDoraSelect
	g.rng = rand.New(rand.NewSource(2))
	tiles := NewShuffledTiles(g.rng)
	_, _, side := DealSplit(tiles)
	g.SideStacks = PairSideWall(side)
	top := g.SideStacks[3][0]

	if _, err := g.ApplySelectDora(1, top.ID); err != ErrNotYourTurn {
		t.Fatalf("non-lead err=%v", err)
	}
	if _, err := g.ApplySelectDora(0, -1); err != ErrInvalidPayload {
		t.Fatalf("bad id err=%v", err)
	}

	ev, err := g.ApplySelectDora(0, top.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseBuild || g.DoraIndicator == nil || g.DoraIndicator.ID != top.ID {
		t.Fatalf("phase=%s dora=%v", g.Phase, g.DoraIndicator)
	}
	if len(g.DoraKinds) != 1 || g.DoraKinds[0] != NextKind(top.Kind) {
		t.Fatalf("doraKinds=%v", g.DoraKinds)
	}
	if g.SideStacks != nil {
		t.Fatal("side stacks should clear")
	}
	if len(ev) != 2 || ev[0].Kind != "DORA" || ev[1].Kind != "PHASE" {
		t.Fatalf("events=%v", eventKinds(ev))
	}
	if g.BuildTimeoutSecValue() != DefaultBuildTimeoutSec {
		t.Fatalf("timeout sec=%d", g.BuildTimeoutSecValue())
	}
	if g.BuildRemainingSec() <= 0 {
		t.Fatal("expected remaining build time")
	}

	if _, err := g.ApplySelectDora(0, top.ID); err != ErrIllegalAction {
		t.Fatalf("after build err=%v", err)
	}
}

func TestForceBuildTimeoutLocksUnsubmitted(t *testing.T) {
	g := readyBuildGame(t)
	g.stopBuildTimer()
	ids0 := wallIDs(g.Players[0].Wall, HandSize)
	if _, err := g.ApplySubmitHand(0, ids0); err != nil {
		t.Fatal(err)
	}
	g.BuildEndsAt = time.Now().Add(-time.Millisecond)
	ev := g.ForceBuildTimeout()
	if g.Phase != PhaseWalk {
		t.Fatalf("phase=%s events=%v", g.Phase, eventKinds(ev))
	}
	if !g.Players[1].HandSubmitted {
		t.Fatal("seat1 should be force-submitted")
	}
	if len(g.Players[1].Hand) != HandSize || len(g.Players[1].Pool) != PoolSize {
		t.Fatalf("hand=%d pool=%d", len(g.Players[1].Hand), len(g.Players[1].Pool))
	}
	kinds := eventKinds(ev)
	if len(kinds) < 2 || kinds[0] != "HAND_LOCKED" || kinds[len(kinds)-1] != "PHASE" {
		t.Fatalf("events=%v", kinds)
	}
}

func TestForceBuildTimeoutNoopBeforeDeadline(t *testing.T) {
	g := readyBuildGame(t)
	g.stopBuildTimer()
	g.BuildEndsAt = time.Now().Add(time.Hour)
	if ev := g.ForceBuildTimeout(); len(ev) != 0 {
		t.Fatalf("unexpected %#v", ev)
	}
	if g.Phase != PhaseBuild {
		t.Fatal(g.Phase)
	}
}

func TestBuildTimerHookTriggersForce(t *testing.T) {
	g := readyBuildGame(t)
	g.stopBuildTimer()
	g.BuildTimeoutSec = 1
	g.BuildEndsAt = time.Now().Add(25 * time.Millisecond)
	done := make(chan []Event, 1)
	g.SetUpdateHook(func() {
		done <- g.ForceBuildTimeout()
	})
	g.startBuildTimer()
	select {
	case ev := <-done:
		if g.Phase != PhaseWalk {
			t.Fatalf("phase=%s events=%v", g.Phase, eventKinds(ev))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("build timer hook did not fire")
	}
}

func TestSideTopsAndScoresDisplay(t *testing.T) {
	g := NewGame()
	g.Players[0].Score = 8000
	g.Players[1].Score = 12000
	sc := g.ScoresDisplay()
	if sc[0] != 8000 || sc[1] != 12000 {
		t.Fatalf("%v", sc)
	}
	g.SideStacks = [][2]Tile{
		{{ID: 1, Kind: KindM1}, {ID: 2, Kind: KindM2}},
		{{ID: 3, Kind: KindP1}, {ID: 4, Kind: KindP2}},
	}
	tops := g.SideTops()
	if len(tops) != 2 || tops[0].ID != 1 || tops[1].ID != 3 {
		t.Fatalf("%#v", tops)
	}
}

func TestContainsIDAndKindsOf(t *testing.T) {
	tiles := tilesOf(KindM1, KindM2)
	if !ContainsID(tiles, 1) || ContainsID(tiles, 99) {
		t.Fatal("ContainsID")
	}
	ks := KindsOf(tiles)
	if len(ks) != 2 || ks[0] != KindM1 || ks[1] != KindM2 {
		t.Fatalf("%v", ks)
	}
}
