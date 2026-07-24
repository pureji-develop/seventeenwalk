package game

// Kind is a tile type string matching frontend sprite IDs.
type Kind string

const (
	KindM1 Kind = "m1"
	KindM2 Kind = "m2"
	KindM3 Kind = "m3"
	KindM4 Kind = "m4"
	KindM5 Kind = "m5"
	KindM6 Kind = "m6"
	KindM7 Kind = "m7"
	KindM8 Kind = "m8"
	KindM9 Kind = "m9"

	KindP1 Kind = "p1"
	KindP2 Kind = "p2"
	KindP3 Kind = "p3"
	KindP4 Kind = "p4"
	KindP5 Kind = "p5"
	KindP6 Kind = "p6"
	KindP7 Kind = "p7"
	KindP8 Kind = "p8"
	KindP9 Kind = "p9"

	KindS1 Kind = "s1"
	KindS2 Kind = "s2"
	KindS3 Kind = "s3"
	KindS4 Kind = "s4"
	KindS5 Kind = "s5"
	KindS6 Kind = "s6"
	KindS7 Kind = "s7"
	KindS8 Kind = "s8"
	KindS9 Kind = "s9"

	KindTon   Kind = "ton"
	KindNan   Kind = "nan"
	KindSha   Kind = "sha"
	KindPei   Kind = "pei"
	KindHaku  Kind = "haku"
	KindHatsu Kind = "hatsu"
	KindChun  Kind = "chun"
)

// AllKinds is the 34 unique tile kinds (4 copies each in a full set).
var AllKinds = []Kind{
	KindM1, KindM2, KindM3, KindM4, KindM5, KindM6, KindM7, KindM8, KindM9,
	KindP1, KindP2, KindP3, KindP4, KindP5, KindP6, KindP7, KindP8, KindP9,
	KindS1, KindS2, KindS3, KindS4, KindS5, KindS6, KindS7, KindS8, KindS9,
	KindTon, KindNan, KindSha, KindPei, KindHaku, KindHatsu, KindChun,
}

// Tile is one physical tile in a deal (unique ID within the match round).
type Tile struct {
	ID   int  `json:"id"`
	Kind Kind `json:"kind"`
}

// NextKind returns the dora kind indicated by a dora indicator tile.
// Cycles: m9→m1, p9→p1, s9→s1, ton→nan→sha→pei→ton, haku→hatsu→chun→haku.
func NextKind(k Kind) Kind {
	switch k {
	case KindM1:
		return KindM2
	case KindM2:
		return KindM3
	case KindM3:
		return KindM4
	case KindM4:
		return KindM5
	case KindM5:
		return KindM6
	case KindM6:
		return KindM7
	case KindM7:
		return KindM8
	case KindM8:
		return KindM9
	case KindM9:
		return KindM1
	case KindP1:
		return KindP2
	case KindP2:
		return KindP3
	case KindP3:
		return KindP4
	case KindP4:
		return KindP5
	case KindP5:
		return KindP6
	case KindP6:
		return KindP7
	case KindP7:
		return KindP8
	case KindP8:
		return KindP9
	case KindP9:
		return KindP1
	case KindS1:
		return KindS2
	case KindS2:
		return KindS3
	case KindS3:
		return KindS4
	case KindS4:
		return KindS5
	case KindS5:
		return KindS6
	case KindS6:
		return KindS7
	case KindS7:
		return KindS8
	case KindS8:
		return KindS9
	case KindS9:
		return KindS1
	case KindTon:
		return KindNan
	case KindNan:
		return KindSha
	case KindSha:
		return KindPei
	case KindPei:
		return KindTon
	case KindHaku:
		return KindHatsu
	case KindHatsu:
		return KindChun
	case KindChun:
		return KindHaku
	default:
		return k
	}
}

// IsHonor reports wind/dragon kinds.
func (k Kind) IsHonor() bool {
	switch k {
	case KindTon, KindNan, KindSha, KindPei, KindHaku, KindHatsu, KindChun:
		return true
	default:
		return false
	}
}

// Suit returns "m","p","s" for suited tiles, "" for honors.
func (k Kind) Suit() string {
	if len(k) == 0 || k.IsHonor() {
		return ""
	}
	return string(k[0])
}

// Rank returns 1-9 for suited tiles, 0 for honors.
func (k Kind) Rank() int {
	if k.IsHonor() || len(k) < 2 {
		return 0
	}
	return int(k[1] - '0')
}
