package game

import "testing"

func TestYakuFourHanBoundary(t *testing.T) {
	// No sanshoku: M234 P456 S234 S67 M88 wait S5/S8 → RIICHI+TANYAO+PINFU = 3
	pinfuHand := []Kind{KindM2, KindM3, KindM4, KindP4, KindP5, KindP6, KindS2, KindS3, KindS4, KindS6, KindS7, KindM8, KindM8}
	// Sanshoku: M234 P234 S234 S67 M88 wait S8
	sanshokuHand := []Kind{KindM2, KindM3, KindM4, KindP2, KindP3, KindP4, KindS2, KindS3, KindS4, KindS6, KindS7, KindM8, KindM8}

	cases := []struct {
		name      string
		hand13    []Kind
		win       Kind
		dora      []Kind
		discTurn  int
		winTurn   int
		lastDisc  bool
		riverWin  bool
		wantUnder bool
		wantWin   bool
		minHan    int
		hasYaku   string
		noYaku    string
	}{
		{
			name:      "first_turn_riichi_ippatsu_mangan",
			hand13:    pinfuHand,
			win:       KindS8,
			discTurn:  1,
			winTurn:   0, // 立直前でも一発
			wantWin:   true,
			minHan:    4, // RIICHI+IPPATSU+TANYAO+PINFU
			hasYaku:   "IPPATSU",
		},
		{
			name:      "riichi_only_under",
			hand13:    []Kind{KindM1, KindM2, KindM3, KindM4, KindM5, KindM6, KindP7, KindP8, KindP9, KindS1, KindS2, KindS3, KindS4},
			win:       KindS4,
			discTurn:  2,
			winTurn:   1,
			wantUnder: true,
			minHan:    1,
			hasYaku:   "RIICHI",
			noYaku:    "IPPATSU",
		},
		{
			name:      "riichi_tanyao_pinfu_under",
			hand13:    pinfuHand,
			win:       KindS8,
			discTurn:  3,
			winTurn:   1,
			wantUnder: true,
			minHan:    3,
			hasYaku:   "PINFU",
			noYaku:    "IPPATSU",
		},
		{
			name:      "riichi_tanyao_pinfu_ipatsu_mangan",
			hand13:    pinfuHand,
			win:       KindS8,
			discTurn:  1,
			winTurn:   1,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "IPPATSU",
		},
		{
			name:      "later_turn_no_ippatsu",
			hand13:    pinfuHand,
			win:       KindS8,
			dora:      []Kind{KindS8},
			discTurn:  3,
			winTurn:   2,
			wantWin:   true,
			minHan:    4,
			noYaku:    "IPPATSU",
		},
		{
			name:      "second_discard_no_ippatsu",
			hand13:    pinfuHand,
			win:       KindS8,
			dora:      []Kind{KindS8},
			discTurn:  2,
			winTurn:   0,
			wantWin:   true,
			minHan:    4,
			noYaku:    "IPPATSU",
		},
		{
			name:      "chiitoi_tanyao_riichi_mangan",
			hand13:    []Kind{KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5, KindS6, KindS6, KindS7, KindS7, KindS8},
			win:       KindS8,
			discTurn:  2,
			winTurn:   1,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "CHIITOITSU",
		},
		{
			name:      "chiitoi_riichi_under",
			hand13:    []Kind{KindM2, KindM2, KindM9, KindM9, KindP1, KindP1, KindP9, KindP9, KindS1, KindS1, KindS9, KindS9, KindTon},
			win:       KindTon,
			discTurn:  2,
			winTurn:   1,
			wantUnder: true,
			minHan:    3,
			hasYaku:   "CHIITOITSU",
		},
		{
			name:      "dora_pushes_pinfu_to_mangan",
			hand13:    pinfuHand,
			win:       KindS8,
			dora:      []Kind{KindS8},
			discTurn:  3,
			winTurn:   1,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "DORA",
		},
		{
			name:      "toitoi_yakuhai_mangan",
			hand13:    []Kind{KindM2, KindM2, KindM2, KindP5, KindP5, KindP5, KindS8, KindS8, KindS8, KindHaku, KindHaku, KindChun, KindChun},
			win:       KindHaku,
			discTurn:  2,
			winTurn:   1,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "TOITOI",
		},
		{
			name:      "honitsu_riichi_mangan",
			hand13:    []Kind{KindM1, KindM2, KindM3, KindM4, KindM5, KindM6, KindM7, KindM8, KindM9, KindTon, KindTon, KindHaku, KindHaku},
			win:       KindHaku,
			discTurn:  2,
			winTurn:   1,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "HONITSU",
		},
		{
			name:      "houtei_helps_pinfu_mangan",
			hand13:    pinfuHand,
			win:       KindS8,
			discTurn:  17,
			winTurn:   1,
			lastDisc:  true,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "HOUTEI",
			noYaku:    "IPPATSU",
		},
		{
			name:      "sanshoku_mangan",
			hand13:    sanshokuHand,
			win:       KindS8,
			discTurn:  3,
			winTurn:   1,
			wantWin:   true,
			minHan:    4,
			hasYaku:   "SANSHOKU",
		},
		{
			name:      "furiten_blocks_chiitoi",
			hand13:    []Kind{KindM2, KindM2, KindM3, KindM3, KindP4, KindP4, KindP5, KindP5, KindS6, KindS6, KindS7, KindS7, KindS8},
			win:       KindS8,
			discTurn:  2,
			winTurn:   1,
			riverWin:  true,
			wantWin:   false,
			wantUnder: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hand := tilesOf(tc.hand13...)
			var river []Tile
			if tc.riverWin {
				river = []Tile{{ID: 500, Kind: tc.win}}
			}
			ev, under := EvaluateRon(RonContext{
				Hand:          hand,
				WinningTile:   Tile{ID: 99, Kind: tc.win},
				River:         river,
				DoraKinds:     tc.dora,
				DiscarderTurn: tc.discTurn,
				WinnerTurn:    tc.winTurn,
				IsLastDiscard: tc.lastDisc,
			})
			if under != tc.wantUnder {
				t.Fatalf("under=%v want %v han=%d yaku=%v", under, tc.wantUnder, ev.Han, ev.Yaku)
			}
			if ev.Winning != tc.wantWin {
				t.Fatalf("winning=%v want %v under=%v han=%d yaku=%v", ev.Winning, tc.wantWin, under, ev.Han, ev.Yaku)
			}
			if (tc.wantWin || tc.wantUnder) && tc.minHan > 0 && ev.Han < tc.minHan {
				t.Fatalf("han=%d want >=%d yaku=%v", ev.Han, tc.minHan, ev.Yaku)
			}
			if tc.hasYaku != "" && (ev.Winning || under) {
				assertHasYaku(t, ev, tc.hasYaku)
			}
			if tc.noYaku != "" && (ev.Winning || under) {
				assertNoYaku(t, ev, tc.noYaku)
			}
		})
	}
}

func TestEvaluateRonUnderReturnsYaku(t *testing.T) {
	hand := tilesOf(KindM1, KindM2, KindM3, KindM4, KindM5, KindM6, KindP7, KindP8, KindP9, KindS1, KindS2, KindS3, KindS4)
	ev, under := EvaluateRon(RonContext{
		Hand:        hand,
		WinningTile: Tile{ID: 1, Kind: KindS4},
		WinnerTurn:  1,
	})
	if !under {
		t.Fatalf("expected under, got winning=%v han=%d yaku=%v", ev.Winning, ev.Han, ev.Yaku)
	}
	if ev.Han < 1 {
		t.Fatalf("under path should still report han, got %d yaku=%v", ev.Han, ev.Yaku)
	}
}

func TestIttsuAndIipeikou(t *testing.T) {
	hand2 := tilesOf(KindM1, KindM2, KindM3, KindM4, KindM5, KindM6, KindM7, KindM8, KindM9, KindP2, KindP3, KindP4, KindS2)
	ev2, under2 := EvaluateRon(RonContext{
		Hand:          hand2,
		WinningTile:   Tile{ID: 2, Kind: KindS2},
		DiscarderTurn: 1,
		WinnerTurn:    1,
	})
	if under2 || !ev2.Winning {
		t.Fatalf("expected ittsu mangan, under=%v han=%d yaku=%v", under2, ev2.Han, ev2.Yaku)
	}
	assertHasYaku(t, ev2, "ITTSU")

	// iipeikou: M223344 P234 S67 M88 wait S5/S8
	hand3 := tilesOf(KindM2, KindM2, KindM3, KindM3, KindM4, KindM4, KindP2, KindP3, KindP4, KindS6, KindS7, KindM8, KindM8)
	ev3, under3 := EvaluateRon(RonContext{
		Hand:          hand3,
		WinningTile:   Tile{ID: 3, Kind: KindS8},
		DiscarderTurn: 3,
		WinnerTurn:    1,
	})
	if under3 || !ev3.Winning {
		t.Fatalf("expected iipeikou mangan, under=%v han=%d yaku=%v", under3, ev3.Han, ev3.Yaku)
	}
	assertHasYaku(t, ev3, "IIPEIKOU")
}

func assertHasYaku(t *testing.T, ev Evaluation, name string) {
	t.Helper()
	for _, y := range ev.Yaku {
		if y == name {
			return
		}
	}
	t.Fatalf("expected %s in %v", name, ev.Yaku)
}

func assertNoYaku(t *testing.T, ev Evaluation, name string) {
	t.Helper()
	for _, y := range ev.Yaku {
		if y == name {
			t.Fatalf("did not expect %s in %v", name, ev.Yaku)
		}
	}
}
