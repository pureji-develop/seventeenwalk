package game

// Evaluation holds a completed hand score for a ron attempt.
type Evaluation struct {
	Winning      bool
	Yaku         []string // codes (compat)
	YakuItems    []YakuItem
	Han          int
	Limit        Limit
	YakumanCount int
	Points       int // ロン加点（麻雀標準点・常に子の点数）
}

// YakuItem is one yaku with its han contribution.
type YakuItem struct {
	Code string
	Han  int
}

// Limit is the hand limit class used for point awards.
type Limit string

const (
	LimitNone          Limit = ""
	LimitMangan        Limit = "MANGAN"
	LimitHaneman       Limit = "HANEMAN"
	LimitBaiman        Limit = "BAIMAN"
	LimitSanbaiman     Limit = "SANBAIMAN"
	LimitYakuman       Limit = "YAKUMAN"
	LimitDoubleYakuman Limit = "DOUBLE_YAKUMAN"
	LimitTripleYakuman Limit = "TRIPLE_YAKUMAN"
)

// RonContext is input for automatic ron checks.
type RonContext struct {
	Hand           []Tile
	WinningTile    Tile
	River          []Tile
	DoraKinds      []Kind
	AgariAbandoned bool // after under-4-han ron tile: cannot ron any wait
	IsLead         bool
	DiscarderTurn  int
	WinnerTurn     int  // 和了者の打牌数（参考用）
	IsLastDiscard  bool // 後攻の第17打
}

type mentsu struct {
	a, b, c Kind
	koutsu  bool // true: koutsu/kantsu-like triplet; false: shuntsu
	pair    bool
}

// AllWaits returns kinds that complete a winning form with the 13-tile hand.
func AllWaits(hand []Tile) []Kind {
	if len(hand) != 13 {
		return nil
	}
	seen := map[Kind]bool{}
	var waits []Kind
	for _, k := range AllKinds {
		if IsWinningForm(appendKind(hand, k)) {
			if !seen[k] {
				seen[k] = true
				waits = append(waits, k)
			}
		}
	}
	return waits
}

func appendKind(hand []Tile, k Kind) []Kind {
	out := make([]Kind, 0, len(hand)+1)
	for _, t := range hand {
		out = append(out, t.Kind)
	}
	out = append(out, k)
	return out
}

// IsFuriten reports whether any wait appears in the player's own river.
func IsFuriten(hand []Tile, river []Tile) bool {
	waits := AllWaits(hand)
	riverSet := map[Kind]bool{}
	for _, t := range river {
		riverSet[t.Kind] = true
	}
	for _, w := range waits {
		if riverSet[w] {
			return true
		}
	}
	return false
}

// EvaluateRon checks win form, furiten, blocked waits, yaku and mangan+.
func EvaluateRon(ctx RonContext) (ev Evaluation, underMangan bool) {
	if ctx.AgariAbandoned {
		return Evaluation{}, false
	}
	kinds := appendKind(ctx.Hand, ctx.WinningTile.Kind)
	if !IsWinningForm(kinds) {
		return Evaluation{}, false
	}
	if IsFuriten(ctx.Hand, ctx.River) {
		return Evaluation{}, false
	}

	ev = scoreHand(kinds, ctx)
	if isManganOrAbove(ev) {
		ev.Winning = true
		ev.Points = RonPoints(ev.Limit, ev.YakumanCount)
		return ev, false
	}
	// completed but under 4 han → caller forfeits ALL waits
	return ev, true
}

func isManganOrAbove(ev Evaluation) bool {
	if ev.YakumanCount > 0 {
		return true
	}
	return ev.Han >= 4
}

// RonPoints returns standard Japanese mahjong ron payment (winner award).
// 符は見ない（4翻以上＝満貫〜）。親子差なし（常に子の点数）。
//
//	満貫8000 / 跳満12000 / 倍満16000 / 三倍満24000 / 役満32000×n
func RonPoints(limit Limit, yakumanCount int) int {
	if yakumanCount > 0 {
		return 32000 * yakumanCount
	}
	switch limit {
	case LimitSanbaiman:
		return 24000
	case LimitBaiman:
		return 16000
	case LimitHaneman:
		return 12000
	case LimitMangan:
		return 8000
	case LimitYakuman:
		return 32000
	case LimitDoubleYakuman:
		return 64000
	case LimitTripleYakuman:
		return 96000
	default:
		return 0
	}
}

// PointsHalfForEvaluation is kept for older call sites.
// Deprecated: use RonPoints.
func PointsHalfForEvaluation(ev Evaluation) int {
	return RonPoints(ev.Limit, ev.YakumanCount)
}

// PointsHalfForLimit maps non-yakuman limit → ron points.
func PointsHalfForLimit(limit Limit) int {
	return RonPoints(limit, 0)
}

func limitFromYakumanCount(n int) Limit {
	switch {
	case n >= 3:
		return LimitTripleYakuman
	case n == 2:
		return LimitDoubleYakuman
	case n == 1:
		return LimitYakuman
	default:
		return LimitNone
	}
}

func scoreHand(kinds []Kind, ctx RonContext) Evaluation {
	if IsKokushi(kinds) {
		yc := 1
		yaku := []string{"KOKUSHI"}
		if len(AllWaits(ctx.Hand)) >= 13 {
			yc = 2
			yaku = append(yaku, "KOKUSHI_13")
		}
		return Evaluation{
			Yaku:         yaku,
			Han:          13 * yc,
			Limit:        limitFromYakumanCount(yc),
			YakumanCount: yc,
		}
	}

	if IsChiitoitsu(kinds) {
		return scoreChiitoitsu(kinds, ctx)
	}

	decomps := enumerateDecomps(kinds)
	if len(decomps) == 0 {
		return Evaluation{}
	}

	best := Evaluation{}
	for _, dec := range decomps {
		ev := scoreStandard(kinds, ctx, dec)
		if betterEval(ev, best) {
			best = ev
		}
	}
	return best
}

func betterEval(a, b Evaluation) bool {
	if a.YakumanCount != b.YakumanCount {
		return a.YakumanCount > b.YakumanCount
	}
	// Compare by points.
	ah, bh := RonPoints(a.Limit, a.YakumanCount), RonPoints(b.Limit, b.YakumanCount)
	if ah != bh {
		return ah > bh
	}
	return a.Han > b.Han
}

func scoreChiitoitsu(kinds []Kind, ctx RonContext) Evaluation {
	yaku := []string{"CHIITOITSU"}
	han := 2

	addAlwaysRiichiYaku(&yaku, &han, ctx)

	if isTanyao(kinds) {
		yaku = append(yaku, "TANYAO")
		han++
	}
	if isHonitsu(kinds) {
		yaku = append(yaku, "HONITSU")
		han += 3
	}
	if isChinitsu(kinds) {
		yaku = append(yaku, "CHINITSU")
		han += 6
	}
	if isHonroutou(kinds) {
		yaku = append(yaku, "HONROUTOU")
		han += 2
	}

	// ドラは役の最後
	han += countLabel(kinds, ctx.DoraKinds, &yaku, "DORA")

	if isTsuuiisou(kinds) {
		return Evaluation{Yaku: []string{"TSUUIISOU"}, Han: 13, Limit: LimitYakuman, YakumanCount: 1}
	}
	yc := 0
	limit := limitFromHan(han)
	if han >= 13 {
		yc = 1
		limit = LimitYakuman
	}
	return Evaluation{Yaku: yaku, Han: han, Limit: limit, YakumanCount: yc}
}

func scoreStandard(kinds []Kind, ctx RonContext, dec []mentsu) Evaluation {
	// Yakuman first
	yc, ykNames := detectYakuman(kinds, dec, ctx)
	if yc > 0 {
		return Evaluation{
			Yaku:         ykNames,
			Han:          13 * yc,
			Limit:        limitFromYakumanCount(yc),
			YakumanCount: yc,
		}
	}

	yaku := []string{}
	han := 0

	addAlwaysRiichiYaku(&yaku, &han, ctx)

	// Yakuhai
	jikaze := KindSha
	if ctx.IsLead {
		jikaze = KindTon
	}
	for _, m := range dec {
		if !m.koutsu || m.pair {
			continue
		}
		k := m.a
		switch k {
		case KindTon:
			yaku = append(yaku, "YAKUHAI_BAKAZE")
			han++
			if ctx.IsLead {
				yaku = append(yaku, "YAKUHAI_JIKAZE")
				han++
			}
		case KindSha:
			if jikaze == KindSha {
				yaku = append(yaku, "YAKUHAI_JIKAZE")
				han++
			}
		case KindHaku:
			yaku = append(yaku, "YAKUHAI_HAKU")
			han++
		case KindHatsu:
			yaku = append(yaku, "YAKUHAI_HATSU")
			han++
		case KindChun:
			yaku = append(yaku, "YAKUHAI_CHUN")
			han++
		}
	}

	if isTanyao(kinds) {
		yaku = append(yaku, "TANYAO")
		han++
	}
	if isPinfu(dec, ctx) {
		yaku = append(yaku, "PINFU")
		han++
	}
	if n := countIipeikou(dec); n == 1 {
		yaku = append(yaku, "IIPEIKOU")
		han++
	} else if n >= 2 {
		yaku = append(yaku, "RYANPEIKOU")
		han += 3
	}
	if hasSanshokuDoujun(dec) {
		yaku = append(yaku, "SANSHOKU")
		han += 2
	}
	if hasIttsu(dec) {
		yaku = append(yaku, "ITTSU")
		han += 2
	}
	if hasToitoi(dec) {
		yaku = append(yaku, "TOITOI")
		han += 2
	}
	if n := countAnkou(dec, ctx); n == 3 {
		yaku = append(yaku, "SANANKOU")
		han += 2
	}
	if hasSanshokuDoukou(dec) {
		yaku = append(yaku, "SANSHOKUDOUKOU")
		han += 2
	}
	if hasShousangen(dec) {
		yaku = append(yaku, "SHOUSANGEN")
		han += 2
	}
	if isChanta(dec) {
		yaku = append(yaku, "CHANTA")
		han += 2
	}
	if isJunchan(dec) {
		yaku = append(yaku, "JUNCHAN")
		han += 3
	}
	if isHonroutou(kinds) {
		yaku = append(yaku, "HONROUTOU")
		han += 2
	}
	if isHonitsu(kinds) {
		yaku = append(yaku, "HONITSU")
		han += 3
	}
	if isChinitsu(kinds) {
		yaku = append(yaku, "CHINITSU")
		han += 6
	}

	// ドラは役の最後
	han += countLabel(kinds, ctx.DoraKinds, &yaku, "DORA")

	yc = 0
	limit := limitFromHan(han)
	if han >= 13 {
		yc = 1
		limit = LimitYakuman
	}
	return Evaluation{Yaku: yaku, Han: han, Limit: limit, YakumanCount: yc}
}

// addAlwaysRiichiYaku: 常時リーチ。打牌者の第1打なら一発、後攻17打目なら河底。
func addAlwaysRiichiYaku(yaku *[]string, han *int, ctx RonContext) {
	*yaku = append(*yaku, "RIICHI")
	*han++
	if isIppatsu(ctx) {
		*yaku = append(*yaku, "IPPATSU")
		*han++
	}
	if ctx.IsLastDiscard {
		*yaku = append(*yaku, "HOUTEI")
		*han++
	}
}

// isIppatsu: 一ターン目（先攻・後攻どちらの第1打でも）のあがり。立直前でも一発。
func isIppatsu(ctx RonContext) bool {
	return ctx.DiscarderTurn == 1
}

func detectYakuman(kinds []Kind, dec []mentsu, ctx RonContext) (int, []string) {
	yc := 0
	var names []string
	add := func(name string, n int) {
		names = append(names, name)
		yc += n
	}

	if countAnkou(dec, ctx) == 4 {
		add("SUUANKOU", 1)
	}
	if dragonKoutsuCount(dec) == 3 {
		add("DAISANGEN", 1)
	}
	winds := windKoutsuCount(dec)
	if winds == 4 {
		add("DAISUUSHII", 2)
	} else if winds == 3 && hasWindPair(dec) {
		add("SHOUSUUSHII", 1)
	}
	if isTsuuiisou(kinds) {
		add("TSUUIISOU", 1)
	}
	if isRyuuiisou(kinds) {
		add("RYUUIISOU", 1)
	}
	if isChinroutou(kinds) {
		add("CHINROUTOU", 1)
	}
	if isChuuren(kinds) {
		add("CHUUREN", 1)
		if isChuurenNineWait(ctx.Hand) {
			add("CHUUREN_9", 1)
		}
	}
	return yc, names
}

func countLabel(kinds []Kind, labels []Kind, yaku *[]string, name string) int {
	set := map[Kind]bool{}
	for _, d := range labels {
		set[d] = true
	}
	n := 0
	for _, k := range kinds {
		if set[k] {
			n++
		}
	}
	if n > 0 {
		*yaku = append(*yaku, name)
	}
	return n
}

func addYaku(items *[]YakuItem, code string, han int) {
	*items = append(*items, YakuItem{Code: code, Han: han})
}

func yakuCodes(items []YakuItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Code
	}
	return out
}

func evalFromItems(items []YakuItem, han int, limit Limit, yc int) Evaluation {
	return Evaluation{
		Yaku:         yakuCodes(items),
		YakuItems:    items,
		Han:          han,
		Limit:        limit,
		YakumanCount: yc,
	}
}

// ExpandYakuItems builds per-yaku han list from codes + total han (ドラは末尾).
func ExpandYakuItems(codes []string, totalHan int) []YakuItem {
	fixed := map[string]int{
		"RIICHI": 1, "IPPATSU": 1, "HOUTEI": 1, "TANYAO": 1, "PINFU": 1,
		"IIPEIKOU": 1, "RYANPEIKOU": 3, "SANSHOKU": 2, "ITTSU": 2, "TOITOI": 2,
		"SANANKOU": 2, "SANSHOKUDOUKOU": 2, "SHOUSANGEN": 2, "CHANTA": 2,
		"JUNCHAN": 3, "HONROUTOU": 2, "HONITSU": 3, "CHINITSU": 6, "CHIITOITSU": 2,
		"YAKUHAI_BAKAZE": 1, "YAKUHAI_JIKAZE": 1, "YAKUHAI_HAKU": 1,
		"YAKUHAI_HATSU": 1, "YAKUHAI_CHUN": 1,
		"KOKUSHI": 13, "KOKUSHI_13": 13, "SUUANKOU": 13, "DAISANGEN": 13,
		"DAISUUSHII": 26, "SHOUSUUSHII": 13, "TSUUIISOU": 13, "RYUUIISOU": 13,
		"CHINROUTOU": 13, "CHUUREN": 13, "CHUUREN_9": 13,
	}
	var items []YakuItem
	used := 0
	hasDora := false
	for _, c := range codes {
		if c == "DORA" || c == "URA_DORA" {
			// URA_DORA は仕様上無し。旧データが来ても無視し、表ドラのみ末尾へ。
			if c == "DORA" {
				hasDora = true
			}
			continue
		}
		h := fixed[c]
		if h == 0 {
			h = 1
		}
		items = append(items, YakuItem{Code: c, Han: h})
		used += h
	}
	remain := totalHan - used
	if remain < 0 {
		remain = 0
	}
	if hasDora {
		items = append(items, YakuItem{Code: "DORA", Han: remain})
	}
	return items
}

func isPinfu(dec []mentsu, ctx RonContext) bool {
	jikaze := KindSha
	if ctx.IsLead {
		jikaze = KindTon
	}
	pairOK := false
	shuntsu := 0
	for _, m := range dec {
		if m.pair {
			k := m.a
			if k == KindTon || k == jikaze || k == KindHaku || k == KindHatsu || k == KindChun {
				return false
			}
			pairOK = true
			continue
		}
		if m.koutsu {
			return false
		}
		shuntsu++
	}
	if !pairOK || shuntsu != 4 {
		return false
	}
	return !isTankiPenkanWait(dec, ctx.WinningTile.Kind)
}

func isTankiPenkanWait(dec []mentsu, win Kind) bool {
	for _, m := range dec {
		if m.pair && m.a == win {
			return true
		}
		if m.koutsu || m.pair {
			continue
		}
		r1, r2, r3 := m.a.Rank(), m.b.Rank(), m.c.Rank()
		if m.a.Suit() == "" {
			continue
		}
		if win == m.c && r1 == 1 && r2 == 2 && r3 == 3 {
			return true
		}
		if win == m.a && r1 == 7 && r2 == 8 && r3 == 9 {
			return true
		}
		if win == m.b {
			return true
		}
	}
	return false
}

func countIipeikou(dec []mentsu) int {
	type key struct{ a, b, c Kind }
	seen := map[key]int{}
	for _, m := range dec {
		if m.pair || m.koutsu {
			continue
		}
		k := key{m.a, m.b, m.c}
		seen[k]++
	}
	pairs := 0
	for _, n := range seen {
		pairs += n / 2
	}
	return pairs
}

func hasSanshokuDoujun(dec []mentsu) bool {
	bySuit := map[string]map[seq]bool{}
	for _, m := range dec {
		if m.pair || m.koutsu {
			continue
		}
		s := m.a.Suit()
		if s == "" {
			continue
		}
		sq := seq{m.a.Rank(), m.b.Rank(), m.c.Rank()}
		if bySuit[s] == nil {
			bySuit[s] = map[seq]bool{}
		}
		bySuit[s][sq] = true
	}
	for sq := range bySuit["m"] {
		if bySuit["p"][sq] && bySuit["s"][sq] {
			return true
		}
	}
	return false
}

type seq struct{ r1, r2, r3 int }

func hasIttsu(dec []mentsu) bool {
	for _, suit := range []string{"m", "p", "s"} {
		has := map[int]bool{}
		for _, m := range dec {
			if m.pair || m.koutsu || m.a.Suit() != suit {
				continue
			}
			if m.a.Rank() == 1 && m.b.Rank() == 2 && m.c.Rank() == 3 {
				has[1] = true
			}
			if m.a.Rank() == 4 && m.b.Rank() == 5 && m.c.Rank() == 6 {
				has[4] = true
			}
			if m.a.Rank() == 7 && m.b.Rank() == 8 && m.c.Rank() == 9 {
				has[7] = true
			}
		}
		if has[1] && has[4] && has[7] {
			return true
		}
	}
	return false
}

func hasToitoi(dec []mentsu) bool {
	for _, m := range dec {
		if !m.pair && !m.koutsu {
			return false
		}
	}
	return true
}

func countAnkou(dec []mentsu, ctx RonContext) int {
	n := 0
	for _, m := range dec {
		if !m.koutsu || m.pair {
			continue
		}
		// ロン牌で完成した刻子は明刻扱い
		if m.a == ctx.WinningTile.Kind {
			continue
		}
		n++
	}
	return n
}

func hasSanshokuDoukou(dec []mentsu) bool {
	ranks := map[int]map[string]bool{}
	for _, m := range dec {
		if !m.koutsu || m.pair || m.a.IsHonor() {
			continue
		}
		r := m.a.Rank()
		if ranks[r] == nil {
			ranks[r] = map[string]bool{}
		}
		ranks[r][m.a.Suit()] = true
	}
	for _, s := range ranks {
		if s["m"] && s["p"] && s["s"] {
			return true
		}
	}
	return false
}

func dragonKoutsuCount(dec []mentsu) int {
	n := 0
	for _, m := range dec {
		if m.koutsu && (m.a == KindHaku || m.a == KindHatsu || m.a == KindChun) {
			n++
		}
	}
	return n
}

func hasShousangen(dec []mentsu) bool {
	koutsu := 0
	pair := false
	for _, m := range dec {
		if m.a != KindHaku && m.a != KindHatsu && m.a != KindChun {
			continue
		}
		if m.pair {
			pair = true
		}
		if m.koutsu {
			koutsu++
		}
	}
	return koutsu == 2 && pair
}

func windKoutsuCount(dec []mentsu) int {
	n := 0
	for _, m := range dec {
		if m.koutsu && isWind(m.a) {
			n++
		}
	}
	return n
}

func hasWindPair(dec []mentsu) bool {
	for _, m := range dec {
		if m.pair && isWind(m.a) {
			return true
		}
	}
	return false
}

func isWind(k Kind) bool {
	return k == KindTon || k == KindNan || k == KindSha || k == KindPei
}

func isTerminalOrHonor(k Kind) bool {
	if k.IsHonor() {
		return true
	}
	r := k.Rank()
	return r == 1 || r == 9
}

func isChanta(dec []mentsu) bool {
	hasHonor := false
	for _, m := range dec {
		var ks []Kind
		if m.pair {
			ks = []Kind{m.a}
		} else {
			ks = []Kind{m.a, m.b, m.c}
		}
		ok := false
		for _, k := range ks {
			if isTerminalOrHonor(k) {
				ok = true
				if k.IsHonor() {
					hasHonor = true
				}
			}
		}
		if !ok {
			return false
		}
	}
	return hasHonor
}

func isJunchan(dec []mentsu) bool {
	for _, m := range dec {
		var ks []Kind
		if m.pair {
			ks = []Kind{m.a}
		} else {
			ks = []Kind{m.a, m.b, m.c}
		}
		ok := false
		for _, k := range ks {
			if k.IsHonor() {
				return false
			}
			r := k.Rank()
			if r == 1 || r == 9 {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func isTanyao(kinds []Kind) bool {
	for _, k := range kinds {
		if isTerminalOrHonor(k) {
			return false
		}
	}
	return true
}

func isHonitsu(kinds []Kind) bool {
	suit := ""
	hasHonor := false
	for _, k := range kinds {
		if k.IsHonor() {
			hasHonor = true
			continue
		}
		if suit == "" {
			suit = k.Suit()
		} else if k.Suit() != suit {
			return false
		}
	}
	return suit != "" && hasHonor
}

func isChinitsu(kinds []Kind) bool {
	suit := ""
	for _, k := range kinds {
		if k.IsHonor() {
			return false
		}
		if suit == "" {
			suit = k.Suit()
		} else if k.Suit() != suit {
			return false
		}
	}
	return suit != ""
}

func isHonroutou(kinds []Kind) bool {
	hasHonor := false
	for _, k := range kinds {
		if !isTerminalOrHonor(k) {
			return false
		}
		if k.IsHonor() {
			hasHonor = true
		}
	}
	return hasHonor
}

func isChinroutou(kinds []Kind) bool {
	for _, k := range kinds {
		if k.IsHonor() {
			return false
		}
		r := k.Rank()
		if r != 1 && r != 9 {
			return false
		}
	}
	return true
}

func isTsuuiisou(kinds []Kind) bool {
	for _, k := range kinds {
		if !k.IsHonor() {
			return false
		}
	}
	return true
}

func isRyuuiisou(kinds []Kind) bool {
	ok := map[Kind]bool{
		KindS2: true, KindS3: true, KindS4: true, KindS6: true, KindS8: true, KindHatsu: true,
	}
	for _, k := range kinds {
		if !ok[k] {
			return false
		}
	}
	return true
}

func isChuuren(kinds []Kind) bool {
	if !isChinitsu(kinds) {
		return false
	}
	c := CountKinds(kinds)
	suit := ""
	for k := range c {
		suit = k.Suit()
		break
	}
	need := []int{3, 1, 1, 1, 1, 1, 1, 1, 3}
	extra := 0
	for r := 1; r <= 9; r++ {
		k := kindFromSuitRank(suit, r)
		n := c[k]
		if n < need[r-1] {
			return false
		}
		extra += n - need[r-1]
	}
	return extra == 1
}

func isChuurenNineWait(hand []Tile) bool {
	// 13枚が 1112345678999 ちょうど
	kinds := make([]Kind, len(hand))
	for i, t := range hand {
		kinds[i] = t.Kind
	}
	if !isChinitsu(kinds) || len(kinds) != 13 {
		return false
	}
	c := CountKinds(kinds)
	suit := kinds[0].Suit()
	need := []int{3, 1, 1, 1, 1, 1, 1, 1, 3}
	for r := 1; r <= 9; r++ {
		if c[kindFromSuitRank(suit, r)] != need[r-1] {
			return false
		}
	}
	return true
}

func limitFromHan(han int) Limit {
	switch {
	case han >= 13:
		return LimitYakuman
	case han >= 11:
		return LimitSanbaiman
	case han >= 8:
		return LimitBaiman
	case han >= 6:
		return LimitHaneman
	case han >= 4:
		return LimitMangan
	default:
		return LimitNone
	}
}

// IsWinningForm reports whether 14 kinds form a complete hand.
func IsWinningForm(kinds []Kind) bool {
	if len(kinds) != 14 {
		return false
	}
	if IsKokushi(kinds) || IsChiitoitsu(kinds) {
		return true
	}
	return len(enumerateDecomps(kinds)) > 0
}

// IsChiitoitsu is seven distinct pairs.
func IsChiitoitsu(kinds []Kind) bool {
	c := CountKinds(kinds)
	if len(c) != 7 {
		return false
	}
	for _, n := range c {
		if n != 2 {
			return false
		}
	}
	return true
}

// IsKokushi is thirteen orphans + pair.
func IsKokushi(kinds []Kind) bool {
	need := []Kind{
		KindM1, KindM9, KindP1, KindP9, KindS1, KindS9,
		KindTon, KindNan, KindSha, KindPei, KindHaku, KindHatsu, KindChun,
	}
	c := CountKinds(kinds)
	pair := false
	for _, k := range need {
		n := c[k]
		if n == 0 || n > 2 {
			return false
		}
		if n == 2 {
			if pair {
				return false
			}
			pair = true
		}
		delete(c, k)
	}
	return pair && len(c) == 0
}

func enumerateDecomps(kinds []Kind) [][]mentsu {
	c := CountKinds(kinds)
	var out [][]mentsu
	for k, n := range c {
		if n >= 2 {
			c[k] -= 2
			rest := mentsuAll(cloneCounts(c))
			c[k] += 2
			for _, ms := range rest {
				dec := append([]mentsu{{a: k, pair: true}}, ms...)
				if len(dec) == 5 {
					out = append(out, dec)
				}
			}
		}
	}
	return out
}

func mentsuAll(c map[Kind]int) [][]mentsu {
	var next Kind
	found := false
	for _, k := range AllKinds {
		if c[k] > 0 {
			next = k
			found = true
			break
		}
	}
	if !found {
		return [][]mentsu{{}}
	}
	var out [][]mentsu
	if c[next] >= 3 {
		c[next] -= 3
		for _, rest := range mentsuAll(c) {
			out = append(out, append([]mentsu{{a: next, b: next, c: next, koutsu: true}}, rest...))
		}
		c[next] += 3
	}
	if suit := next.Suit(); suit != "" {
		r := next.Rank()
		if r >= 1 && r <= 7 {
			b := kindFromSuitRank(suit, r+1)
			cc := kindFromSuitRank(suit, r+2)
			if c[next] > 0 && c[b] > 0 && c[cc] > 0 {
				c[next]--
				c[b]--
				c[cc]--
				for _, rest := range mentsuAll(c) {
					out = append(out, append([]mentsu{{a: next, b: b, c: cc}}, rest...))
				}
				c[next]++
				c[b]++
				c[cc]++
			}
		}
	}
	return out
}

func cloneCounts(c map[Kind]int) map[Kind]int {
	x := make(map[Kind]int, len(c))
	for k, n := range c {
		if n > 0 {
			x[k] = n
		}
	}
	return x
}

func kindFromSuitRank(suit string, rank int) Kind {
	return Kind(suit + string(rune('0'+rank)))
}
