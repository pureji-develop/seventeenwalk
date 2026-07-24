package game

import "testing"

func TestLeadRonRotatesLeadAndAdvancesRound(t *testing.T) {
	g := NewGame()
	g.JoinSeat("a", "A")
	g.JoinSeat("b", "B")
	g.ApplyReady(0, true)
	g.ApplyReady(1, true)
	lead := g.LeadSeat
	w := lead
	l := Seat(1 - lead)
	g.Phase = PhaseRoundEnd
	g.Result = &RoundResult{Result: "RON", WinnerSeat: &w, LoserSeat: &l}
	g.Round = 1
	g.TotalRounds = 4
	g.ApplyAckRound(0)
	g.ApplyAckRound(1)
	if g.LeadSeat == lead {
		t.Fatalf("no renchan: lead should rotate from %d", lead)
	}
	if g.Round != 2 {
		t.Fatalf("round should advance to 2, got %d", g.Round)
	}
}

func TestDrawRotatesLead(t *testing.T) {
	g := NewGame()
	g.JoinSeat("a", "A")
	g.JoinSeat("b", "B")
	g.ApplyReady(0, true)
	g.ApplyReady(1, true)
	lead := g.LeadSeat
	g.Phase = PhaseRoundEnd
	g.Result = &RoundResult{Result: "DRAW"}
	g.Round = 1
	g.ApplyAckRound(0)
	g.ApplyAckRound(1)
	if g.LeadSeat == lead {
		t.Fatalf("draw should rotate lead from %d", lead)
	}
	if g.Round != 2 {
		t.Fatalf("draw should advance round to 2, got %d", g.Round)
	}
}

func TestUnderManganAbandonsAllWaits(t *testing.T) {
	g := NewGame()
	g.Phase = PhaseWalk
	g.LeadSeat = 0
	g.ToAct = 0
	// seat1 has a hand that can win on s7 (under 4) or would win later — after under, abandon all
	g.Players[1].Hand = tilesOf(
		KindM1, KindM1, KindM2, KindM2, KindP3, KindP3, KindP4, KindP4,
		KindS5, KindS5, KindS6, KindS6, KindS7,
	)
	g.Players[0].Pool = []Tile{{ID: 200, Kind: KindS7}, {ID: 201, Kind: KindM8}}
	g.Players[0].TurnCount = 2 // after discard → 3, avoid ippatsu making it 4 han
	_, err := g.ApplyDiscard(0, 200)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Players[1].AgariAbandoned {
		t.Fatal("expected AgariAbandoned after under-4-han ron tile")
	}
	// even if a 4+ han tile were discarded next, cannot ron
	g.ToAct = 0
	g.Players[1].Hand = tilesOf(
		KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5,
		KindS6, KindS6, KindS7, KindS7, KindS8,
	)
	ev, under := EvaluateRon(RonContext{
		Hand:           g.Players[1].Hand,
		WinningTile:    Tile{ID: 201, Kind: KindS8},
		AgariAbandoned: g.Players[1].AgariAbandoned,
	})
	if under || ev.Winning {
		t.Fatalf("abandoned player must not ron, got under=%v ev=%#v", under, ev)
	}
}

func TestManganAutoRon(t *testing.T) {
	// riichi + chiitoi + tanyao = 4
	hand := tilesOf(
		KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5,
		KindS6, KindS6, KindS7, KindS7, KindS8,
	)
	win := Tile{ID: 99, Kind: KindS8}
	ev, under := EvaluateRon(RonContext{
		Hand:        hand,
		WinningTile: win,
		IsLead:      false,
		WinnerTurn:  1,
	})
	if under || !ev.Winning {
		t.Fatalf("expected mangan ron, under=%v ev=%#v", under, ev)
	}
	if ev.Limit != LimitMangan {
		t.Fatalf("limit=%s han=%d yaku=%v", ev.Limit, ev.Han, ev.Yaku)
	}
}

func tilesOf(ks ...Kind) []Tile {
	out := make([]Tile, len(ks))
	for i, k := range ks {
		out[i] = Tile{ID: i + 1, Kind: k}
	}
	return out
}

func TestParseTimeoutPreset(t *testing.T) {
	cases := []struct {
		preset string
		fb     int
		want   int
		ok     bool
	}{
		{"inf", 180, 0, true},
		{"INF", 180, 0, true},
		{"120", 0, 120, true},
		{"180", 0, 180, true},
		{"300", 0, 300, true},
		{"", 0, 0, true},
		{"", 180, 180, true},
		{"", 99, 0, false},
		{"90", 0, 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseTimeoutPreset(tc.preset, tc.fb)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseTimeoutPreset(%q,%d)=(%d,%v) want (%d,%v)", tc.preset, tc.fb, got, ok, tc.want, tc.ok)
		}
	}
}

func TestApplySetOptionsInfinite(t *testing.T) {
	g := NewGame()
	g.JoinSeat("a", "A")
	g.Players[0].Ready = true
	events, err := g.ApplySetOptions(0, 0, 6, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.BuildTimeoutSec != 0 || g.BuildTimeout != 0 || !g.WallSorted || g.TotalRounds != 6 {
		t.Fatalf("options not applied: sec=%d dur=%v wall=%v rounds=%d", g.BuildTimeoutSec, g.BuildTimeout, g.WallSorted, g.TotalRounds)
	}
	if g.Players[0].Ready {
		t.Fatal("ready should clear")
	}
	if len(events) == 0 || events[0].Kind != "OPTIONS" {
		t.Fatalf("expected OPTIONS event, got %#v", events)
	}
}

func TestNoKyoutakuOnFirstDiscard(t *testing.T) {
	g := NewGame()
	g.JoinSeat("a", "A")
	g.JoinSeat("b", "B")
	g.Phase = PhaseWalk
	g.LeadSeat = 0
	g.ToAct = 0
	g.Players[0].Pool = []Tile{{ID: 1, Kind: KindM1}, {ID: 2, Kind: KindM2}}
	g.Players[1].Hand = tilesOf(
		KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5,
		KindS6, KindS6, KindS7, KindS7, KindS8,
	)
	events, err := g.ApplyDiscard(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Kind == "RIICHI" {
			t.Fatal("riichi/kyoutaku events are abolished")
		}
	}
	if len(g.Players[0].River) != 1 {
		t.Fatal("river")
	}
}

func TestFirstDiscardRonHasIppatsuNoKyoutaku(t *testing.T) {
	g := NewGame()
	g.Phase = PhaseWalk
	g.LeadSeat = 0
	g.ToAct = 0
	g.Players[1].TurnCount = 0
	g.Players[0].Pool = []Tile{{ID: 1, Kind: KindS8}, {ID: 2, Kind: KindM1}}
	g.Players[1].Hand = tilesOf(
		KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5,
		KindS6, KindS6, KindS7, KindS7, KindS8,
	)
	events, err := g.ApplyDiscard(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != PhaseRoundEnd || g.Result == nil || g.Result.Result != "RON" {
		t.Fatalf("expected ron, phase=%s events=%v", g.Phase, eventKinds(events))
	}
	hasIppatsu := false
	for _, y := range g.Result.Yaku {
		if y == "IPPATSU" {
			hasIppatsu = true
		}
		if y == "RIICHI" {
			// always present
		}
	}
	if !hasIppatsu {
		t.Fatalf("expected IPPATSU on first-turn ron, yaku=%v", g.Result.Yaku)
	}
	hasRiichi := false
	for _, y := range g.Result.Yaku {
		if y == "RIICHI" {
			hasRiichi = true
		}
	}
	if !hasRiichi {
		t.Fatalf("expected always RIICHI, yaku=%v", g.Result.Yaku)
	}
	if g.Players[1].Score != g.Result.HandPoints || g.Result.HandPoints < 8000 {
		t.Fatalf("score=%d handPoints=%d yaku=%v han=%d (no kyoutaku)", g.Players[1].Score, g.Result.HandPoints, g.Result.Yaku, g.Result.Han)
	}
}

func eventKinds(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func TestFinishRonNoKyoutaku(t *testing.T) {
	g := NewGame()
	g.Phase = PhaseWalk
	g.LeadSeat = 0
	g.Players[1].Hand = tilesOf(
		KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5,
		KindS6, KindS6, KindS7, KindS7, KindS8,
	)
	ev := Evaluation{Winning: true, Limit: LimitMangan, Han: 4, Points: 8000, Yaku: []string{"RIICHI"}}
	_ = g.finishRon(1, 0, Tile{ID: 99, Kind: KindS8}, ev)
	if g.Players[1].Score != 8000 {
		t.Fatalf("score=%d want 8000", g.Players[1].Score)
	}
	if g.Result.HandPoints != 8000 {
		t.Fatalf("handPoints=%d", g.Result.HandPoints)
	}
}
