package protocol

import (
	"testing"

	"seventeenwalk/internal/game"
)

func TestBuildStateDrawRevealsOpponentTiles(t *testing.T) {
	g := game.NewGame()
	g.JoinSeat("a", "A")
	g.JoinSeat("b", "B")
	g.Phase = game.PhaseRoundEnd
	g.Result = &game.RoundResult{Result: "DRAW"}
	g.Players[0].Hand = []game.Tile{{ID: 1, Kind: game.KindM1}, {ID: 2, Kind: game.KindM2}}
	g.Players[0].Pool = []game.Tile{{ID: 3, Kind: game.KindM3}}
	g.Players[1].Hand = []game.Tile{{ID: 10, Kind: game.KindP1}, {ID: 11, Kind: game.KindP2}}
	g.Players[1].Pool = []game.Tile{{ID: 12, Kind: game.KindP3}, {ID: 13, Kind: game.KindP4}}

	st := BuildState(g, "room", 0)
	if len(st.Opponent.Hand) != 2 {
		t.Fatalf("expected opponent hand revealed, got %d", len(st.Opponent.Hand))
	}
	if len(st.Opponent.Pool) != 2 {
		t.Fatalf("expected opponent pool revealed, got %d", len(st.Opponent.Pool))
	}
	if len(st.You.Hand) != 2 || len(st.You.Pool) != 1 {
		t.Fatalf("expected self hand/pool, hand=%d pool=%d", len(st.You.Hand), len(st.You.Pool))
	}
}

func TestBuildStateRonRevealsOpponentTiles(t *testing.T) {
	g := game.NewGame()
	g.JoinSeat("a", "A")
	g.JoinSeat("b", "B")
	g.Phase = game.PhaseRoundEnd
	w := game.Seat(0)
	g.Result = &game.RoundResult{Result: "RON", WinnerSeat: &w}
	g.Players[0].Hand = []game.Tile{{ID: 1, Kind: game.KindM1}}
	g.Players[0].Pool = []game.Tile{{ID: 2, Kind: game.KindM2}}
	g.Players[1].Hand = []game.Tile{{ID: 10, Kind: game.KindP1}, {ID: 11, Kind: game.KindP2}}
	g.Players[1].Pool = []game.Tile{{ID: 12, Kind: game.KindP3}}

	// 自分が和了しても相手の手牌・残牌が見える（盤面 peek 用）
	st := BuildState(g, "room", 0)
	if len(st.Opponent.Hand) != 2 || len(st.Opponent.Pool) != 1 {
		t.Fatalf("expected opp reveal on RON, hand=%d pool=%d", len(st.Opponent.Hand), len(st.Opponent.Pool))
	}
}
