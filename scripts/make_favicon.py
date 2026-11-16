#!/usr/bin/env python3
"""中（chun）牌画像から favicon を生成する。

Usage:
  python3 scripts/make_favicon.py
  python3 scripts/make_favicon.py --sizes 16 32 48
"""

from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SRC = ROOT / "gazou" / "tupai2_1" / "p_ji_c_1.gif"
DEFAULT_OUT_DIR = ROOT / "frontend" / "public"
DEFAULT_SIZES = (16, 32, 48)
PWA_SIZES = (192, 512)
APPLE_SIZE = 180
FELT_BG = (12, 46, 36, 255)  # #0c2e24


def load_chun(path: Path) -> Image.Image:
    with Image.open(path) as img:
        return img.convert("RGBA")


def make_square(tile: Image.Image, size: int, pad_ratio: float = 0.08) -> Image.Image:
    """牌を正方形キャンバス中央に配置（透過背景）。"""
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    pad = max(0, int(size * pad_ratio))
    target = size - pad * 2
    # 牌のアスペクトを保ったまま収める
    scale = min(target / tile.width, target / tile.height)
    tw = max(1, int(round(tile.width * scale)))
    th = max(1, int(round(tile.height * scale)))
    resized = tile.resize((tw, th), Image.Resampling.LANCZOS)
    x = (size - tw) // 2
    y = (size - th) // 2
    canvas.paste(resized, (x, y), resized)
    return canvas


def make_felt_icon(tile: Image.Image, size: int, pad_ratio: float = 0.18) -> Image.Image:
    """Felt-green square icon for PWA / home screen (maskable-friendly)."""
    canvas = Image.new("RGBA", (size, size), FELT_BG)
    pad = max(0, int(size * pad_ratio))
    target = size - pad * 2
    scale = min(target / tile.width, target / tile.height)
    tw = max(1, int(round(tile.width * scale)))
    th = max(1, int(round(tile.height * scale)))
    resized = tile.resize((tw, th), Image.Resampling.LANCZOS)
    x = (size - tw) // 2
    y = (size - th) // 2
    canvas.paste(resized, (x, y), resized)
    return canvas


def write_pngs(tile: Image.Image, out_dir: Path, sizes: list[int]) -> list[Path]:
    out_dir.mkdir(parents=True, exist_ok=True)
    written: list[Path] = []
    for s in sizes:
        path = out_dir / f"favicon-{s}x{s}.png"
        make_square(tile, s).save(path, format="PNG", optimize=True)
        written.append(path)
    # 既定の参照用（32px）
    primary = out_dir / "favicon.png"
    make_square(tile, 32).save(primary, format="PNG", optimize=True)
    written.append(primary)
    for s in PWA_SIZES:
        path = out_dir / f"pwa-{s}x{s}.png"
        make_felt_icon(tile, s).save(path, format="PNG", optimize=True)
        written.append(path)
    apple = out_dir / "apple-touch-icon.png"
    make_felt_icon(tile, APPLE_SIZE, pad_ratio=0.14).save(apple, format="PNG", optimize=True)
    written.append(apple)
    return written


def write_ico(tile: Image.Image, out_path: Path, sizes: list[int]) -> Path:
    """マルチサイズ ICO（ブラウザ向け）。"""
    frames = [make_square(tile, s) for s in sorted(set(sizes))]
    # Pillow: 最大サイズを本体、sizes= で埋め込み
    largest = max(frames, key=lambda im: im.width)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    largest.save(
        out_path,
        format="ICO",
        sizes=[(im.width, im.height) for im in frames],
        append_images=frames,
    )
    return out_path


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate favicon from 中 tile GIF")
    parser.add_argument(
        "--src",
        type=Path,
        default=DEFAULT_SRC,
        help=f"source tile image (default: {DEFAULT_SRC})",
    )
    parser.add_argument(
        "--out-dir",
        type=Path,
        default=DEFAULT_OUT_DIR,
        help=f"output directory (default: {DEFAULT_OUT_DIR})",
    )
    parser.add_argument(
        "--sizes",
        type=int,
        nargs="+",
        default=list(DEFAULT_SIZES),
        help="PNG / ICO sizes in pixels",
    )
    args = parser.parse_args()

    if not args.src.exists():
        raise SystemExit(f"source not found: {args.src}")

    tile = load_chun(args.src)
    pngs = write_pngs(tile, args.out_dir, args.sizes)
    ico = write_ico(tile, args.out_dir / "favicon.ico", args.sizes)

    print("favicon generated:")
    for p in pngs:
        print(f"  {p.relative_to(ROOT)}")
    print(f"  {ico.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
