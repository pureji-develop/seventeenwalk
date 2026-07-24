package game

import "testing"

func TestNextKind(t *testing.T) {
	cases := map[Kind]Kind{
		KindM9:    KindM1,
		KindP9:    KindP1,
		KindS9:    KindS1,
		KindM5:    KindM6,
		KindTon:   KindNan,
		KindNan:   KindSha,
		KindSha:   KindPei,
		KindPei:   KindTon,
		KindHaku:  KindHatsu,
		KindHatsu: KindChun,
		KindChun:  KindHaku,
	}
	for in, want := range cases {
		if got := NextKind(in); got != want {
			t.Fatalf("NextKind(%s)=%s want %s", in, got, want)
		}
	}
}

func TestRonPointsTable(t *testing.T) {
	cases := []struct {
		limit Limit
		yc    int
		want  int
	}{
		{LimitMangan, 0, 8000},
		{LimitHaneman, 0, 12000},
		{LimitBaiman, 0, 16000},
		{LimitSanbaiman, 0, 24000},
		{LimitYakuman, 1, 32000},
		{LimitDoubleYakuman, 2, 64000},
		{LimitTripleYakuman, 3, 96000},
	}
	for _, tc := range cases {
		if got := RonPoints(tc.limit, tc.yc); got != tc.want {
			t.Fatalf("%s yc=%d got %d want %d", tc.limit, tc.yc, got, tc.want)
		}
	}
}

func TestPointsHalfForLimitCompat(t *testing.T) {
	if PointsHalfForLimit(LimitMangan) != 8000 {
		t.Fatal("mangan child")
	}
	if PointsHalfForEvaluation(Evaluation{YakumanCount: 2, Limit: LimitDoubleYakuman}) != 64000 {
		t.Fatal("double yakuman child")
	}
}
