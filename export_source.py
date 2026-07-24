#!/usr/bin/env python3
"""Export text sources into zzz_codes/{frontend,backend,design}.txt (each starts with a tree)."""

import os
from pathlib import Path

EXCLUDE_DIRS = {
    ".git",
    "node_modules",
    "__pycache__",
    "dist",
    "build",
    ".DS_Store",
    ".idea",
    ".vscode",
    "zzz_codes",
}

OUTPUT_DIR = "zzz_codes"

# Root folder → output file name
TARGETS = (
    ("frontend", "frontend.txt"),
    ("backend", "backend.txt"),
    ("00_design", "design.txt"),
)


def is_text_file(filepath: Path) -> bool:
    try:
        with open(filepath, "r", encoding="utf-8") as check_file:
            check_file.read()
        return True
    except (UnicodeDecodeError, OSError):
        return False


def build_tree(root: Path, exclude_dirs: set[str]) -> str:
    """ASCII tree of root, skipping exclude_dirs and non-text files."""
    lines = [f"{root.name}/"]

    def walk(directory: Path, prefix: str) -> None:
        try:
            entries = sorted(directory.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower()))
        except OSError:
            return

        visible: list[Path] = []
        for entry in entries:
            if entry.name in exclude_dirs:
                continue
            if entry.is_symlink():
                continue
            if entry.is_dir():
                visible.append(entry)
            elif entry.is_file() and is_text_file(entry):
                visible.append(entry)

        for i, entry in enumerate(visible):
            last = i == len(visible) - 1
            branch = "└── " if last else "├── "
            lines.append(f"{prefix}{branch}{entry.name}{'/' if entry.is_dir() else ''}")
            if entry.is_dir():
                extension = "    " if last else "│   "
                walk(entry, prefix + extension)

    walk(root, "")
    return "\n".join(lines) + "\n"


def export_folder(folder: Path, output: Path, exclude_dirs: set[str], exclude_files: set[str]) -> int:
    if not folder.is_dir():
        print(f"Skip (missing): {folder}")
        return 0

    folder_label = folder.name
    count = 0
    with open(output, "w", encoding="utf-8") as out:
        out.write(f"{'=' * 60}\n")
        out.write(f"Tree: {folder_label}/\n")
        out.write(f"{'=' * 60}\n")
        out.write(build_tree(folder, exclude_dirs))
        out.write("\n")

        for dirpath, dirnames, filenames in os.walk(folder):
            dirnames[:] = sorted(d for d in dirnames if d not in exclude_dirs)
            filenames = sorted(filenames)

            for filename in filenames:
                if filename in exclude_files:
                    continue

                filepath = Path(dirpath) / filename
                if filepath.is_symlink():
                    continue
                if not is_text_file(filepath):
                    continue

                try:
                    content = filepath.read_text(encoding="utf-8")
                except OSError as e:
                    print(f"Failed to read {filepath}: {e}")
                    continue

                try:
                    rel = filepath.relative_to(folder.parent).as_posix()
                except ValueError:
                    rel = filepath.as_posix()

                out.write(f"\n{'=' * 60}\n")
                out.write(f"File: {rel}\n")
                out.write(f"{'=' * 60}\n")
                out.write(content)
                if not content.endswith("\n"):
                    out.write("\n")
                count += 1

    return count


def main() -> None:
    root = Path(__file__).resolve().parent
    os.chdir(root)

    out_dir = root / OUTPUT_DIR
    out_dir.mkdir(parents=True, exist_ok=True)

    exclude_files = {Path(__file__).name} | {name for _, name in TARGETS}
    # Also ignore legacy single-file export if present
    exclude_files.add("source.txt")

    total = 0
    for folder_name, out_name in TARGETS:
        out_path = out_dir / out_name
        n = export_folder(root / folder_name, out_path, EXCLUDE_DIRS, exclude_files)
        print(f"{OUTPUT_DIR}/{out_name}: {n} files from {folder_name}/")
        total += n

    print(f"Done! Exported {total} text files → {OUTPUT_DIR}/{{frontend,backend,design}}.txt")


if __name__ == "__main__":
    main()
