package game

import (
	"fmt"
	"strings"
	"testing"
)

type yakumanCase struct {
	name     string
	hand13   []Kind
	win      Kind
	wantMin  int
	wantYaku string
}

func TestYakumanHundred(t *testing.T) {
	cases := make([]yakumanCase, 0, 150)
	cases = append(cases, kokushiCases()...)
	cases = append(cases, kokushi13Cases()...)
	cases = append(cases, suuankouCases()...)
	cases = append(cases, daisangenCases()...)
	cases = append(cases, shousuushiiCases()...)
	cases = append(cases, daisuushiiCases()...)
	cases = append(cases, tsuuiisouCases()...)
	cases = append(cases, ryuuiisouCases()...)
	cases = append(cases, chinroutouCases()...)
	cases = append(cases, chuurenCases()...)
	cases = append(cases, chuuren9Cases()...)

	if len(cases) < 100 {
		t.Fatalf("need >=100 cases, got %d", len(cases))
	}
	t.Logf("running %d yakuman cases", len(cases))

	pass, fail := 0, 0
	for i, tc := range cases {
		tc := tc
		ok := t.Run(fmt.Sprintf("%03d_%s", i+1, sanitizeName(tc.name)), func(t *testing.T) {
			if len(tc.hand13) != 13 {
				t.Fatalf("hand13 len=%d", len(tc.hand13))
			}
			hand := make([]Tile, 13)
			for j, k := range tc.hand13 {
				hand[j] = Tile{ID: j + 1, Kind: k}
			}
			all := append(append([]Kind(nil), tc.hand13...), tc.win)
			if !IsWinningForm(all) {
				t.Fatalf("not winning form: %v + %v", tc.hand13, tc.win)
			}
			ev, under := EvaluateRon(RonContext{
				Hand:          hand,
				WinningTile:   Tile{ID: 99, Kind: tc.win},
				IsLead:        false,
				DiscarderTurn: 5,
			})
			if under || !ev.Winning {
				t.Fatalf("expected yakuman win, under=%v ev=%#v", under, ev)
			}
			if ev.YakumanCount < tc.wantMin {
				t.Fatalf("YakumanCount=%d want>=%d yaku=%v", ev.YakumanCount, tc.wantMin, ev.Yaku)
			}
			if tc.wantYaku != "" && !hasYaku(ev.Yaku, tc.wantYaku) {
				t.Fatalf("missing %s in %v", tc.wantYaku, ev.Yaku)
			}
		})
		if ok {
			pass++
		} else {
			fail++
		}
	}
	t.Logf("summary pass=%d fail=%d total=%d", pass, fail, len(cases))
}

func hasYaku(yaku []string, name string) bool {
	for _, y := range yaku {
		if y == name {
			return true
		}
	}
	return false
}

func sanitizeName(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "_"), "/", "_")
}

var orphans = []Kind{
	KindM1, KindM9, KindP1, KindP9, KindS1, KindS9,
	KindTon, KindNan, KindSha, KindPei, KindHaku, KindHatsu, KindChun,
}

func kokushiCases() []yakumanCase {
	var out []yakumanCase
	for pi, pairKind := range orphans {
		for wi, winKind := range orphans {
			if pi == wi {
				continue
			}
			hand := []Kind{pairKind, pairKind}
			for j, o := range orphans {
				if j == pi || j == wi {
					continue
				}
				hand = append(hand, o)
			}
			if len(hand) != 13 {
				continue
			}
			out = append(out, yakumanCase{
				name:     fmt.Sprintf("kokushi_pair_%s_win_%s", pairKind, winKind),
				hand13:   hand,
				win:      winKind,
				wantMin:  1,
				wantYaku: "KOKUSHI",
			})
			if len(out) >= 20 {
				return out
			}
		}
	}
	return out
}

func kokushi13Cases() []yakumanCase {
	var out []yakumanCase
	base := append([]Kind(nil), orphans...)
	for _, win := range orphans {
		out = append(out, yakumanCase{
			name:     fmt.Sprintf("kokushi13_win_%s", win),
			hand13:   append([]Kind(nil), base...),
			win:      win,
			wantMin:  2,
			wantYaku: "KOKUSHI_13",
		})
	}
	return out
}

func suuankouCases() []yakumanCase {
	type set struct {
		name string
		kot  [4]Kind
		pair Kind
	}
	sets := []set{
		{"m1234", [4]Kind{KindM1, KindM2, KindM3, KindM4}, KindM5},
		{"m5678", [4]Kind{KindM5, KindM6, KindM7, KindM8}, KindM9},
		{"p1234", [4]Kind{KindP1, KindP2, KindP3, KindP4}, KindP5},
		{"p5678", [4]Kind{KindP5, KindP6, KindP7, KindP8}, KindP9},
		{"s1234", [4]Kind{KindS1, KindS2, KindS3, KindS4}, KindS5},
		{"s5678", [4]Kind{KindS5, KindS6, KindS7, KindS8}, KindS9},
		{"mix1", [4]Kind{KindM1, KindP2, KindS3, KindTon}, KindNan},
		{"mix2", [4]Kind{KindM9, KindP9, KindS9, KindSha}, KindPei},
		{"mix3", [4]Kind{KindM2, KindM4, KindM6, KindM8}, KindP5},
		{"hon1", [4]Kind{KindTon, KindNan, KindSha, KindHaku}, KindHatsu},
		{"hon2", [4]Kind{KindHaku, KindHatsu, KindChun, KindTon}, KindM5},
		{"mid1", [4]Kind{KindM3, KindP3, KindS3, KindM7}, KindP7},
	}
	var out []yakumanCase
	for _, s := range sets {
		hand := make([]Kind, 0, 13)
		for _, k := range s.kot {
			hand = append(hand, k, k, k)
		}
		hand = append(hand, s.pair)
		out = append(out, yakumanCase{
			name: "suuankou_" + s.name, hand13: hand, win: s.pair,
			wantMin: 1, wantYaku: "SUUANKOU",
		})
	}
	return out
}

func daisangenCases() []yakumanCase {
	pairs := []Kind{KindM2, KindM5, KindP3, KindP8, KindS4, KindS6, KindTon, KindNan}
	fourths := []Kind{KindM1, KindP9, KindS2, KindSha, KindM3, KindP4, KindS8, KindPei}
	var out []yakumanCase
	for i, pair := range pairs {
		fk := fourths[i]
		hand := []Kind{
			KindHaku, KindHaku, KindHaku,
			KindHatsu, KindHatsu, KindHatsu,
			KindChun, KindChun, KindChun,
			fk, fk, pair, pair,
		}
		out = append(out, yakumanCase{
			name: fmt.Sprintf("daisangen_%s_%s", pair, fk), hand13: hand, win: fk,
			wantMin: 1, wantYaku: "DAISANGEN",
		})
	}
	return out
}

func shousuushiiCases() []yakumanCase {
	type conf struct {
		name  string
		winds [3]Kind
		wpair Kind
		other Kind
	}
	confs := []conf{
		{"a", [3]Kind{KindTon, KindNan, KindSha}, KindPei, KindM5},
		{"b", [3]Kind{KindTon, KindNan, KindPei}, KindSha, KindP3},
		{"c", [3]Kind{KindTon, KindSha, KindPei}, KindNan, KindS7},
		{"d", [3]Kind{KindNan, KindSha, KindPei}, KindTon, KindM2},
		{"e", [3]Kind{KindTon, KindNan, KindSha}, KindPei, KindHaku},
		{"f", [3]Kind{KindTon, KindNan, KindPei}, KindSha, KindChun},
	}
	var out []yakumanCase
	for _, c := range confs {
		hand := []Kind{c.wpair, c.wpair}
		for _, w := range c.winds {
			hand = append(hand, w, w, w)
		}
		hand = append(hand, c.other, c.other)
		out = append(out, yakumanCase{
			name: "shousuushii_" + c.name, hand13: hand, win: c.other,
			wantMin: 1, wantYaku: "SHOUSUUSHII",
		})
	}
	return out
}

func daisuushiiCases() []yakumanCase {
	pairs := []Kind{KindM3, KindM5, KindP2, KindP8, KindS4, KindS6, KindHaku, KindHatsu, KindChun, KindM9}
	var out []yakumanCase
	for _, pair := range pairs {
		hand := []Kind{
			KindTon, KindTon, KindTon,
			KindNan, KindNan, KindNan,
			KindSha, KindSha, KindSha,
			KindPei, KindPei, pair, pair,
		}
		out = append(out, yakumanCase{
			name: fmt.Sprintf("daisuushii_%s", pair), hand13: hand, win: KindPei,
			wantMin: 2, wantYaku: "DAISUUSHII",
		})
	}
	return out
}

func tsuuiisouCases() []yakumanCase {
	var out []yakumanCase
	chiis := []struct {
		h   []Kind
		win Kind
	}{
		{[]Kind{KindTon, KindTon, KindNan, KindNan, KindSha, KindSha, KindPei, KindPei, KindHaku, KindHaku, KindHatsu, KindHatsu, KindChun}, KindChun},
		{[]Kind{KindTon, KindTon, KindNan, KindNan, KindSha, KindSha, KindPei, KindPei, KindHaku, KindHaku, KindChun, KindChun, KindHatsu}, KindHatsu},
		{[]Kind{KindTon, KindTon, KindNan, KindNan, KindSha, KindSha, KindHaku, KindHaku, KindHatsu, KindHatsu, KindChun, KindChun, KindPei}, KindPei},
	}
	for i, c := range chiis {
		out = append(out, yakumanCase{
			name: fmt.Sprintf("tsuuiisou_chiitoi_%d", i), hand13: c.h, win: c.win,
			wantMin: 1, wantYaku: "TSUUIISOU",
		})
	}
	out = append(out, yakumanCase{
		name: "tsuuiisou_std_1",
		hand13: []Kind{KindTon, KindTon, KindTon, KindNan, KindNan, KindNan, KindSha, KindSha, KindSha, KindHaku, KindHaku, KindHaku, KindHatsu},
		win: KindHatsu, wantMin: 1, wantYaku: "TSUUIISOU",
	})
	out = append(out, yakumanCase{
		name: "tsuuiisou_std_2",
		hand13: []Kind{KindPei, KindPei, KindPei, KindNan, KindNan, KindNan, KindSha, KindSha, KindSha, KindChun, KindChun, KindChun, KindTon},
		win: KindTon, wantMin: 1, wantYaku: "TSUUIISOU",
	})
	return out
}

func ryuuiisouCases() []yakumanCase {
	configs := []struct {
		name string
		h    []Kind
		win  Kind
	}{
		{"a", []Kind{KindS2, KindS2, KindS2, KindS3, KindS3, KindS3, KindS4, KindS4, KindS4, KindS6, KindS6, KindS6, KindS8}, KindS8},
		{"b", []Kind{KindS2, KindS2, KindS2, KindS3, KindS3, KindS3, KindS4, KindS4, KindS4, KindS8, KindS8, KindS8, KindHatsu}, KindHatsu},
		{"c", []Kind{KindS2, KindS2, KindS2, KindS6, KindS6, KindS6, KindS8, KindS8, KindS8, KindHatsu, KindHatsu, KindHatsu, KindS3}, KindS3},
		{"d", []Kind{KindS3, KindS3, KindS3, KindS4, KindS4, KindS4, KindS6, KindS6, KindS6, KindHatsu, KindHatsu, KindHatsu, KindS2}, KindS2},
		{"e", []Kind{KindS2, KindS2, KindS2, KindS3, KindS4, KindS4, KindS4, KindS6, KindS6, KindS6, KindS8, KindS8, KindS8}, KindS3},
		{"f", []Kind{KindS2, KindS2, KindS2, KindS3, KindS3, KindS3, KindS6, KindS6, KindS6, KindS8, KindS8, KindS8, KindS4}, KindS4},
		{"g", []Kind{KindS4, KindS4, KindS4, KindS6, KindS6, KindS6, KindS8, KindS8, KindS8, KindHatsu, KindHatsu, KindHatsu, KindS3}, KindS3},
		{"h", []Kind{KindS2, KindS2, KindS2, KindS4, KindS4, KindS4, KindS6, KindS6, KindS6, KindHatsu, KindHatsu, KindHatsu, KindS8}, KindS8},
	}
	var out []yakumanCase
	for _, c := range configs {
		out = append(out, yakumanCase{
			name: "ryuuiisou_" + c.name, hand13: c.h, win: c.win,
			wantMin: 1, wantYaku: "RYUUIISOU",
		})
	}
	return out
}

func chinroutouCases() []yakumanCase {
	terminals := []Kind{KindM1, KindM9, KindP1, KindP9, KindS1, KindS9}
	var out []yakumanCase
	for i := 0; i < 6; i++ {
		pair := terminals[i]
		kots := make([]Kind, 0, 4)
		for j, t := range terminals {
			if j == i {
				continue
			}
			kots = append(kots, t)
			if len(kots) == 4 {
				break
			}
		}
		hand := []Kind{pair}
		for _, k := range kots {
			hand = append(hand, k, k, k)
		}
		out = append(out, yakumanCase{
			name: fmt.Sprintf("chinroutou_tanki_%s", pair), hand13: hand, win: pair,
			wantMin: 1, wantYaku: "CHINROUTOU",
		})
	}
	for i := 0; i < 6; i++ {
		pair := terminals[(i+1)%6]
		kotWin := terminals[i]
		hand := []Kind{pair, pair, kotWin, kotWin}
		n := 0
		for _, t := range terminals {
			if t == pair || t == kotWin {
				continue
			}
			hand = append(hand, t, t, t)
			n++
			if n == 3 {
				break
			}
		}
		out = append(out, yakumanCase{
			name: fmt.Sprintf("chinroutou_ronkot_%s", kotWin), hand13: hand, win: kotWin,
			wantMin: 1, wantYaku: "CHINROUTOU",
		})
	}
	return out
}

func chuurenCases() []yakumanCase {
	var out []yakumanCase
	for _, suit := range []string{"m", "p", "s"} {
		k := func(r int) Kind { return kindFromSuitRank(suit, r) }
		hand := []Kind{k(1), k(1), k(1), k(2), k(2), k(3), k(4), k(5), k(6), k(7), k(8), k(9), k(9)}
		out = append(out, yakumanCase{
			name: fmt.Sprintf("chuuren_%s_extra2_win9", suit), hand13: hand, win: k(9),
			wantMin: 1, wantYaku: "CHUUREN",
		})
		base := []Kind{k(1), k(1), k(1), k(2), k(3), k(4), k(5), k(6), k(7), k(8), k(9), k(9), k(9)}
		for winRank := 1; winRank <= 9; winRank++ {
			out = append(out, yakumanCase{
				name: fmt.Sprintf("chuuren_%s_win%d", suit, winRank), hand13: append([]Kind(nil), base...), win: k(winRank),
				wantMin: 1, wantYaku: "CHUUREN",
			})
		}
	}
	return out
}

func chuuren9Cases() []yakumanCase {
	var out []yakumanCase
	for _, suit := range []string{"m", "p", "s"} {
		k := func(r int) Kind { return kindFromSuitRank(suit, r) }
		hand := []Kind{k(1), k(1), k(1), k(2), k(3), k(4), k(5), k(6), k(7), k(8), k(9), k(9), k(9)}
		for winRank := 1; winRank <= 9; winRank++ {
			out = append(out, yakumanCase{
				name: fmt.Sprintf("chuuren9_%s_win%d", suit, winRank), hand13: append([]Kind(nil), hand...), win: k(winRank),
				wantMin: 2, wantYaku: "CHUUREN_9",
			})
		}
	}
	return out
}
