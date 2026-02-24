"""Pretty terminal output helper (example)

Usage:
  python scripts/pretty_output.py --provinces 5

This script uses `rich` to print a colored table of the first N provinces
from `data/v2/province.json` (default 10). It demonstrates colored headers,
row striping, and a simple status color mapping.
"""
from __future__ import annotations
import json
from pathlib import Path
import argparse
from rich.console import Console
from rich.table import Table
from rich import box
from rich.style import Style

DATA_DIR = Path(__file__).parent.parent / "data" / "v2"

STATUS_COLOR = {
    "active": "green",
    "merged": "yellow",
    "split": "magenta",
    "renamed": "cyan",
    "": "white",
}


def load_provinces(path: Path):
    pfile = path / "province.json"
    with pfile.open(encoding="utf-8") as f:
        return json.load(f)


def build_table(provinces: list[dict], limit: int = 10) -> Table:
    table = Table(title=f"Provinces (first {limit})", box=box.SIMPLE_HEAVY)
    table.add_column("Code", justify="right", style="bold")
    table.add_column("ID", style="dim")
    table.add_column("Name")
    table.add_column("Region", style="cyan")
    table.add_column("Pop.", justify="right")
    table.add_column("Status")

    for p in provinces[:limit]:
        status = p.get("status", "")
        status_color = STATUS_COLOR.get(status, "white")
        name = p.get("name", "")
        # highlight merged provinces
        if status == "merged":
            name = f"[strike]{name}[/]"
        table.add_row(
            str(p.get("code", "")),
            p.get("id", ""),
            name,
            p.get("region", ""),
            f"{p.get('population', 0):,}",
            f"[{status_color}]{status or 'active'}[/]",
        )
    return table


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--provinces", "-n", type=int, default=10, help="number of provinces to show")
    args = parser.parse_args()

    console = Console()
    provinces = load_provinces(DATA_DIR)
    table = build_table(provinces, limit=args.provinces)
    console.print(table)


if __name__ == "__main__":
    main()
