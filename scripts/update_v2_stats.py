"""
Update v2/province.json with aggregated stats from v1 sources, and strip
the `population` field from v1/province.json.

Rules
-----
For each v2 province that has `merged_from` codes:
  - area_km2   = sum of area_km2 from all v1 sources (self + merged)
  - population = sum of population from all v1 sources (self + merged)
  - boundaries = union of all v1 source boundaries, then:
      * replace v1 province names with their v2 equivalents
      * remove names that refer to provinces now merged into the same v2 unit
      * deduplicate, preserve a stable order

For v2 provinces with no `merged_from` (already covered by their own v1 record):
  - area_km2 / population are kept as-is (already correct from v1)
  - boundaries are kept as-is

v1/province.json:
  - Remove the `population` field from every entry

Run:
    python scripts/update_v2_stats.py
"""

from __future__ import annotations

import json
from pathlib import Path

REPO_ROOT = Path(__file__).parent.parent
V1_FILE = REPO_ROOT / "data" / "v1" / "province.json"
V2_FILE = REPO_ROOT / "data" / "v2" / "province.json"
MAP_FILE = REPO_ROOT / "data" / "map.json"

# Aliases for v1 boundary strings that use non-standard or abbreviated names.
# Maps the alias (as found in boundaries) -> canonical v1 province name
V1_BOUNDARY_ALIASES: dict[str, str] = {
    "TP.HCM":    "TP. Hồ Chí Minh",       # abbreviated form used in some v1 files
    "Vũng Tàu":  "Bà Rịa - Vũng Tàu",     # old short name
    "Hòa Bình":  "Hoà Bình",               # NFC vs NFD / different combining chars
    "Khánh Hòa": "Khánh Hoà",
    "Thanh Hóa": "Thanh Hoá",              # same unicode normalisation issue
}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def load(path: Path) -> list:
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def save(path: Path, data: list) -> None:
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    print(f"  Written -> {path.relative_to(REPO_ROOT)}")


# ---------------------------------------------------------------------------
# Build v1 lookup: code (int) -> province entry
# ---------------------------------------------------------------------------

def build_v1_by_code(v1: list) -> dict[int, dict]:
    return {p["code"]: p for p in v1 if "code" in p}


# ---------------------------------------------------------------------------
# Build name-translation map: v1 short name -> v2 full name
# and a set of v2-internal names (provinces absorbed into same v2 unit)
# ---------------------------------------------------------------------------

def build_name_maps(
    v2: list,
    v1_by_code: dict[int, dict],
    v2_to_v1_codes: dict[int, list[int]],
) -> tuple[dict[str, str], dict[int, set[str]]]:
    """
    Returns:
      v1_name_to_v2_name  : v1 province short name -> v2 province full name
      self_internal_names : v2_code -> set of v1 short names that are internal
                            to *that specific* v2 unit (absorbed, not neighbours)
    """
    # Map every v1 code -> v2 province name (using map.json source of truth)
    code_to_v2_name: dict[int, str] = {}
    for v2p in v2:
        v2_code = v2p["code"]
        v2_name = v2p["name"]
        for v1_code in v2_to_v1_codes.get(v2_code, [v2_code]):
            code_to_v2_name[v1_code] = v2_name

    # v1 short name -> v2 full name
    v1_name_to_v2_name: dict[str, str] = {}
    for code, v1p in v1_by_code.items():
        v2_name = code_to_v2_name.get(code)
        if v2_name:
            v1_name_to_v2_name[v1p["name"]] = v2_name

    # Also add aliases so non-standard boundary strings get translated too
    for alias, canonical in V1_BOUNDARY_ALIASES.items():
        if canonical in v1_name_to_v2_name:
            v1_name_to_v2_name[alias] = v1_name_to_v2_name[canonical]

    # self_internal_names: for each v2 province, the set of v1 SHORT names
    # that are part of THIS specific merge (must be dropped from boundaries).
    # These are NOT neighbours — they are now interior to the new unit.
    # Also include known aliases for those names.
    self_internal_names: dict[int, set[str]] = {}
    for v2p in v2:
        v2_code = v2p["code"]
        source_codes = v2_to_v1_codes.get(v2_code, [v2_code])
        names: set[str] = set()
        for c in source_codes:
            v1p = v1_by_code.get(c)
            if v1p:
                canonical = v1p["name"]
                names.add(canonical)
                # Add any aliases that map to this canonical name
                for alias, can in V1_BOUNDARY_ALIASES.items():
                    if can == canonical:
                        names.add(alias)
        self_internal_names[v2_code] = names

    return v1_name_to_v2_name, self_internal_names


# ---------------------------------------------------------------------------
# Compute merged boundaries for a v2 province
# ---------------------------------------------------------------------------

def merge_boundaries(
    source_codes: list[int],
    v2_code: int,
    v1_by_code: dict[int, dict],
    v1_name_to_v2_name: dict[str, str],
    self_internal_names: dict[int, set[str]],
    self_v2_name: str,
) -> list[str]:
    """
    Union the boundaries of all v1 source provinces, then:
    1. Skip v1 names that belong to the SAME v2 unit (internal provinces, not neighbours).
    2. Translate remaining v1 names to their v2 equivalents.
    3. Skip self-references (translated name == this v2 province's name).
    4. Deduplicate, preserve order of first occurrence.
    """
    my_internals = self_internal_names.get(v2_code, set())

    seen: list[str] = []
    seen_set: set[str] = set()

    for code in source_codes:
        v1p = v1_by_code.get(code, {})
        for raw_name in v1p.get("boundaries", []):
            # Skip if this v1 name is a province absorbed into THIS same v2 unit
            if raw_name in my_internals:
                continue
            # Translate to v2 name (if v1 province was absorbed into another v2 unit)
            translated = v1_name_to_v2_name.get(raw_name, raw_name)
            # Skip self-reference
            if translated == self_v2_name:
                continue
            if not translated:
                continue
            if translated not in seen_set:
                seen.append(translated)
                seen_set.add(translated)

    return seen


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> None:
    print("=== Update v2 stats from v1 sources ===\n")

    v1 = load(V1_FILE)
    v2 = load(V2_FILE)
    mapd = load(MAP_FILE)

    v1_by_code = build_v1_by_code(v1)

    # Build authoritative v2_code -> [v1_codes] from map.json
    # This is the ground truth for which v1 provinces form each v2 province.
    v2_to_v1_codes: dict[int, list[int]] = {}
    for entry in mapd["provinces"]:
        v2_to_v1_codes[entry["v2_code"]] = entry["v1_codes"]

    # Build name maps using the correct v1_codes from map.json
    v1_name_to_v2_name, self_internal_names = build_name_maps(v2, v1_by_code, v2_to_v1_codes)

    updated_v2 = []
    for v2p in v2:
        entry = dict(v2p)
        v2_code: int = v2p["code"]

        # Use map.json as source of truth for v1 codes
        source_codes = v2_to_v1_codes.get(v2_code, [v2_code])

        # Aggregate area and population from all v1 sources
        total_area = 0.0
        total_pop = 0
        for code in source_codes:
            v1p = v1_by_code.get(code, {})
            total_area += v1p.get("area_km2", 0.0) or 0.0
            total_pop += v1p.get("population", 0) or 0

        if total_area > 0:
            entry["area_km2"] = round(total_area, 1) if total_area != int(total_area) else int(total_area)
        if total_pop > 0:
            entry["population"] = total_pop

        # Recompute boundaries:
        # - Always when multiple v1 sources were merged
        # - Also when current v2 boundaries is empty (fill from v1 source)
        if len(source_codes) > 1 or not entry.get("boundaries"):
            new_bounds = merge_boundaries(
                source_codes, v2_code, v1_by_code, v1_name_to_v2_name, self_internal_names, v2p["name"]
            )
            if new_bounds:
                entry["boundaries"] = new_bounds

        updated_v2.append(entry)

    # --- Strip population from v1 (do this AFTER computing v2 stats) ---
    updated_v1 = []
    for p in v1:
        entry = {k: v for k, v in p.items() if k != "population"}
        updated_v1.append(entry)

    save(V2_FILE, updated_v2)
    save(V1_FILE, updated_v1)

    # --- Report ---
    print()
    changed = sum(
        1 for old, new in zip(v2, updated_v2)
        if old.get("area_km2") != new.get("area_km2")
        or old.get("population") != new.get("population")
        or old.get("boundaries") != new.get("boundaries")
    )
    print(f"v2 provinces updated: {changed}/{len(updated_v2)}")
    print(f"v1 population field removed from {len(updated_v1)} entries")

    print("\nSample v2 updates:")
    for old, new in zip(v2, updated_v2):
        if (old.get("area_km2") != new.get("area_km2")
                or old.get("population") != new.get("population")
                or old.get("boundaries") != new.get("boundaries")):
            print(f"  [{new['code']}] {new['id']}")
            print(f"    area:  {old.get('area_km2'):>10} -> {new.get('area_km2')}")
            print(f"    pop:   {old.get('population'):>10} -> {new.get('population')}")
            if old.get("boundaries") != new.get("boundaries"):
                print(f"    bounds -> {new.get('boundaries')}")


if __name__ == "__main__":
    main()
