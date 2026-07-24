/** 役・点数の和文表記（ロン画面用）。 */

const YAKU_JA: Record<string, string> = {
  RIICHI: '立直',
  IPPATSU: '一発',
  HOUTEI: '河底撈魚',
  TANYAO: '断么九',
  PINFU: '平和',
  IIPEIKOU: '一盃口',
  RYANPEIKOU: '二盃口',
  SANSHOKU: '三色同順',
  ITTSU: '一気通貫',
  TOITOI: '対々和',
  SANANKOU: '三暗刻',
  SANSHOKUDOUKOU: '三色同刻',
  SHOUSANGEN: '小三元',
  CHANTA: '混全帯么九',
  JUNCHAN: '純全帯么九',
  HONROUTOU: '混老頭',
  HONITSU: '混一色',
  CHINITSU: '清一色',
  CHIITOITSU: '七対子',
  DORA: 'ドラ',
  YAKUHAI_BAKAZE: '役牌・場風',
  YAKUHAI_JIKAZE: '役牌・自風',
  YAKUHAI_HAKU: '役牌・白',
  YAKUHAI_HATSU: '役牌・發',
  YAKUHAI_CHUN: '役牌・中',
  KOKUSHI: '国士無双',
  KOKUSHI_13: '国士無双・十三面待ち',
  SUUANKOU: '四暗刻',
  DAISANGEN: '大三元',
  DAISUUSHII: '大四喜',
  SHOUSUUSHII: '小四喜',
  TSUUIISOU: '字一色',
  RYUUIISOU: '緑一色',
  CHINROUTOU: '清老頭',
  CHUUREN: '九蓮宝燈',
  CHUUREN_9: '純正九蓮宝燈',
};

const LIMIT_JA: Record<string, string> = {
  MANGAN: '満貫',
  HANEMAN: '跳満',
  BAIMAN: '倍満',
  SANBAIMAN: '三倍満',
  YAKUMAN: '役満',
  DOUBLE_YAKUMAN: '二倍役満',
  TRIPLE_YAKUMAN: '三倍役満',
};

export function yakuLabel(code: string): string {
  return YAKU_JA[code] ?? code;
}

export function formatYakuList(yaku: string[] | null | undefined): string {
  if (!yaku?.length) return '—';
  return yaku.map(yakuLabel).join('　');
}

export function formatYakuLines(yaku: string[] | null | undefined): string[] {
  if (!yaku?.length) return [];
  return yaku.map(yakuLabel);
}

export function formatLimit(limit: string | null | undefined, yakumanCount?: number): string {
  if (!limit) return '';
  const base = LIMIT_JA[limit] ?? limit;
  if (yakumanCount && yakumanCount > 1 && limit === 'YAKUMAN') {
    return `${yakumanCount}倍役満`;
  }
  return base;
}

/** 8000 → "8,000" （和文向けカンマ） */
export function formatPoints(n: number | null | undefined): string {
  const v = Math.round(Number(n) || 0);
  return v.toLocaleString('ja-JP');
}

/** ロン加点の大きな見出し用（例: 8,000点） */
export function formatPointsDisplay(n: number | null | undefined): string {
  return `${formatPoints(n)}点`;
}

/** 役から麻雀標準ロン点を算出（表示フォールバック用・常に子の点数） */
export function ronPointsFromLimit(
  limit: string | null | undefined,
  yakumanCount: number
): number {
  const yc = yakumanCount || 0;
  if (yc > 0 || limit === 'YAKUMAN' || limit === 'DOUBLE_YAKUMAN' || limit === 'TRIPLE_YAKUMAN') {
    const n =
      yc > 0
        ? yc
        : limit === 'TRIPLE_YAKUMAN'
          ? 3
          : limit === 'DOUBLE_YAKUMAN'
            ? 2
            : 1;
    return 32000 * n;
  }
  switch (limit) {
    case 'SANBAIMAN':
      return 24000;
    case 'BAIMAN':
      return 16000;
    case 'HANEMAN':
      return 12000;
    case 'MANGAN':
      return 8000;
    default:
      return 0;
  }
}

export interface YakuRow {
  code: string;
  name: string;
  han: number;
}

/** サーバ yakuDetails 優先。ドラは常に末尾。 */
export function buildYakuRows(
  details: { code: string; han: number; name?: string }[] | null | undefined,
  codes: string[] | null | undefined,
  totalHan: number
): YakuRow[] {
  const toRow = (d: { code: string; han: number; name?: string }): YakuRow => ({
    code: d.code,
    name: d.name?.trim() ? d.name : yakuLabel(d.code),
    han: d.han,
  });

  if (details?.length) {
    return orderDoraLast(details.map(toRow).filter((r) => r.code !== 'URA_DORA'));
  }
  if (!codes?.length) return [];
  const fixed: Record<string, number> = {
    RIICHI: 1,
    IPPATSU: 1,
    HOUTEI: 1,
    TANYAO: 1,
    PINFU: 1,
    IIPEIKOU: 1,
    RYANPEIKOU: 3,
    SANSHOKU: 2,
    ITTSU: 2,
    TOITOI: 2,
    SANANKOU: 2,
    SANSHOKUDOUKOU: 2,
    SHOUSANGEN: 2,
    CHANTA: 2,
    JUNCHAN: 3,
    HONROUTOU: 2,
    HONITSU: 3,
    CHINITSU: 6,
    CHIITOITSU: 2,
    YAKUHAI_BAKAZE: 1,
    YAKUHAI_JIKAZE: 1,
    YAKUHAI_HAKU: 1,
    YAKUHAI_HATSU: 1,
    YAKUHAI_CHUN: 1,
  };
  const rows: YakuRow[] = [];
  let used = 0;
  let hasDora = false;
  for (const c of codes) {
    if (c === 'URA_DORA') continue;
    if (c === 'DORA') {
      hasDora = true;
      continue;
    }
    const h = fixed[c] ?? 1;
    rows.push({ code: c, name: yakuLabel(c), han: h });
    used += h;
  }
  const remain = Math.max(0, totalHan - used);
  if (hasDora) rows.push({ code: 'DORA', name: yakuLabel('DORA'), han: remain });
  return rows;
}

function orderDoraLast(rows: YakuRow[]): YakuRow[] {
  const main: YakuRow[] = [];
  const dora: YakuRow[] = [];
  for (const r of rows) {
    if (r.code === 'DORA') dora.push(r);
    else main.push(r);
  }
  return [...main, ...dora];
}
