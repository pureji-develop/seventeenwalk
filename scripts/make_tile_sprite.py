#!/usr/bin/env python3
"""gazou/ 内の麻雀牌 GIF を WebP スプライトシートにまとめる。

Usage:
  pip install pillow
  python scripts/make_tile_sprite.py
  python scripts/make_tile_sprite.py --columns 9 --padding 0
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path

from PIL import Image

# スプライト上の並び（左→右、上→下）
# 萬子 / 筒子 / 索子 / 字牌(東南西北白發中)
TILE_ORDER: list[tuple[str, str]] = [
    *[(f"manzu2_1/p_ms{i}_1.gif", f"m{i}") for i in range(1, 10)],
    *[(f"pinzu2_1/p_ps{i}_1.gif", f"p{i}") for i in range(1, 10)],
    *[(f"sozu2_1/p_ss{i}_1.gif", f"s{i}") for i in range(1, 10)],
    ("tupai2_1/p_ji_e_1.gif", "ton"),    # 東
    ("tupai2_1/p_ji_s_1.gif", "nan"),    # 南
    ("tupai2_1/p_ji_w_1.gif", "sha"),    # 西
    ("tupai2_1/p_ji_n_1.gif", "pei"),    # 北
    ("tupai2_1/p_ji_a_1.gif", "haku"),   # 白
    ("tupai2_1/p_ji_h_1.gif", "hatsu"),  # 發
    ("tupai2_1/p_ji_c_1.gif", "chun"),   # 中
]


def load_rgba(path: Path) -> Image.Image:
    with Image.open(path) as img:
        return img.convert("RGBA")


def build_sprite(
    gazou_dir: Path,
    columns: int,
    padding: int,
) -> tuple[Image.Image, list[dict], int, int]:
    tiles: list[tuple[str, Image.Image]] = []
    for rel, key in TILE_ORDER:
        path = gazou_dir / rel
        if not path.exists():
            raise FileNotFoundError(f"tile not found: {path}")
        tiles.append((key, load_rgba(path)))

    tile_w = max(im.width for _, im in tiles)
    tile_h = max(im.height for _, im in tiles)

    n = len(tiles)
    cols = max(1, columns)
    rows = math.ceil(n / cols)

    sheet_w = cols * tile_w + (cols + 1) * padding
    sheet_h = rows * tile_h + (rows + 1) * padding
    sheet = Image.new("RGBA", (sheet_w, sheet_h), (0, 0, 0, 0))

    frames: list[dict] = []
    for i, (key, im) in enumerate(tiles):
        col = i % cols
        row = i // cols
        cell_x = padding + col * (tile_w + padding)
        cell_y = padding + row * (tile_h + padding)
        x = cell_x + (tile_w - im.width) // 2
        y = cell_y + (tile_h - im.height) // 2
        sheet.paste(im, (x, y), im)
        frames.append(
            {
                "id": key,
                "index": i,
                "x": x,
                "y": y,
                "w": im.width,
                "h": im.height,
                "cellX": cell_x,
                "cellY": cell_y,
                "cellW": tile_w,
                "cellH": tile_h,
                "col": col,
                "row": row,
            }
        )

    return sheet, frames, tile_w, tile_h


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description="Build mahjong tile WebP sprite")
    parser.add_argument(
        "--input",
        type=Path,
        default=root / "gazou",
        help="牌画像フォルダ (default: ./gazou)",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=root / "frontend" / "public" / "tiles.webp",
        help="出力 WebP パス",
    )
    parser.add_argument(
        "--meta",
        type=Path,
        default=None,
        help="座標 JSON の出力パス (default: output と同名 .json)",
    )
    parser.add_argument(
        "--columns",
        type=int,
        default=9,
        help="1行あたりの牌数 (default: 9 → 萬/筒/索が各1行)",
    )
    parser.add_argument(
        "--padding",
        type=int,
        default=0,
        help="牌同士の余白 px",
    )
    parser.add_argument(
        "--quality",
        type=int,
        default=90,
        help="WebP quality (lossy 時のみ)",
    )
    parser.add_argument(
        "--lossy",
        action="store_true",
        help="非可逆 WebP で保存 (default: lossless)",
    )
    args = parser.parse_args()

    sheet, frames, tile_w, tile_h = build_sprite(
        args.input, columns=args.columns, padding=args.padding
    )

    out = args.output
    out.parent.mkdir(parents=True, exist_ok=True)
    save_kw: dict = {"format": "WEBP", "method": 6}
    if args.lossy:
        save_kw["lossless"] = False
        save_kw["quality"] = args.quality
    else:
        save_kw["lossless"] = True
    sheet.save(out, **save_kw)

    meta_path = args.meta or out.with_suffix(".json")
    meta = {
        "image": out.name,
        "tileWidth": tile_w,
        "tileHeight": tile_h,
        "columns": args.columns,
        "padding": args.padding,
        "sheetWidth": sheet.width,
        "sheetHeight": sheet.height,
        "count": len(frames),
        "frames": frames,
        "byId": {f["id"]: f for f in frames},
    }
    meta_path.write_text(
        json.dumps(meta, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )

    print(f"wrote {out} ({sheet.width}x{sheet.height}, {len(frames)} tiles)")
    print(f"wrote {meta_path}")


if __name__ == "__main__":
    main()
