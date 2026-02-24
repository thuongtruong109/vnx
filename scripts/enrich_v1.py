"""
Enrich v1 JSON data files with additional fields to support address conversion features.

Changes applied:
  1. data/v1/province.json
       - Add "code" (numeric GSO code from map.json v1_codes)
       - Add "status": "active"
       - Add "merged_into" where the province was absorbed by a v2 province

  2. data/v1/<province>.json  (per-province address files)
       - "province_id" → keep as slug (already is slug in most files; normalise)
       - Add "code" to each district (from map.json district_code)
       - Add "status": "active" to each district
       - Add "code" to each ward (from map.json v1 ward codes, matched by name+district)
       - Add "status": "active" to each ward

Run:
    python scripts/enrich_v1.py
"""

from __future__ import annotations

import json
import re
import unicodedata
from collections import defaultdict
from pathlib import Path

REPO_ROOT = Path(__file__).parent.parent
V1_DIR = REPO_ROOT / "data" / "v1"
MAP_FILE = REPO_ROOT / "data" / "map.json"

# Manual slug → v1 province code overrides for provinces whose names don't
# match any map.json v1_names entry (e.g. abbreviations, old names).
SLUG_CODE_OVERRIDES: dict[str, int] = {
    "thuthienhue": 46,   # "Thừa Thiên Huế" → code 46 (now merged into Huế v2)
    "tphcm": 79,         # "TP. Hồ Chí Minh" → code 79
    "laochau": 12,       # "Lai Châu" (id typo in province.json)
    "gia lai": 64,       # "Gia Lai" (id has space in province.json)
}

# -------------------------------------------------------------------------
# Helpers
# -------------------------------------------------------------------------

def normalize(text: str) -> str:
    """Lowercase, strip diacritics, collapse spaces – for fuzzy name matching."""
    text = unicodedata.normalize("NFD", text)
    text = "".join(c for c in text if unicodedata.category(c) != "Mn")
    return re.sub(r"\s+", " ", text).strip().lower()


def load_json(path: Path) -> list | dict:
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def save_json(path: Path, data: list | dict) -> None:
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    print(f"  Written -> {path.relative_to(REPO_ROOT)}")


# -------------------------------------------------------------------------
# Build lookup tables from map.json
# -------------------------------------------------------------------------

def build_lookups(map_data: dict):
    """
    Returns:
      province_code_by_name  : normalize(v1_name) → v1 numeric code
      province_v2_by_v1code  : v1_code → {v2_code, v2_id, v2_name}
      district_code_by_norm  : (province_code, normalize(district_name)) → district_code
      ward_code_by_norm      : (province_code, normalize(district_name), normalize(ward_name)) → ward_code
      all_v1_codes           : set of all valid v1 province codes (ints)
    """
    province_code_by_name: dict[str, int] = {}
    province_v2_by_v1code: dict[int, dict] = {}
    all_v1_codes: set[int] = set()

    for entry in map_data["provinces"]:
        v2_info = {
            "v2_code": entry["v2_code"],
            "v2_id": entry["v2_id"],
            "v2_name": entry["v2_name"],
        }
        for code, name in zip(entry["v1_codes"], entry["v1_names"]):
            province_code_by_name[normalize(name)] = code
            province_v2_by_v1code[code] = v2_info
            all_v1_codes.add(code)

    # Ward map: ward_code_by_norm[(province_code, district_norm, ward_norm)] = code
    # Also build district_code_by_norm[(province_code, district_norm)] = district_code
    # Use a "first seen" approach – one ward can appear in multiple v2 wards; just
    # record its code once (codes are stable v1 identifiers).
    district_code_by_norm: dict[tuple, int] = {}
    ward_code_by_norm: dict[tuple, int] = {}

    for ward_entry in map_data["wards"]:
        for v1w in ward_entry["v1_wards"]:
            pcode = v1w["province_code"]
            dcode = v1w["district_code"]
            dname_norm = normalize(v1w["district_name"])
            wname_norm = normalize(v1w["name"])

            all_v1_codes.add(pcode)  # collect all v1 province codes seen in ward data
            district_code_by_norm.setdefault((pcode, dname_norm), dcode)
            ward_code_by_norm.setdefault((pcode, dname_norm, wname_norm), v1w["code"])

    return (
        province_code_by_name,
        province_v2_by_v1code,
        district_code_by_norm,
        ward_code_by_norm,
        all_v1_codes,
    )


# -------------------------------------------------------------------------
# Build province code by slug from province.json itself + map data
# -------------------------------------------------------------------------

def build_province_slug_to_code(
    provinces: list[dict],
    province_code_by_name: dict[str, int],
) -> dict[str, int]:
    """
    Returns slug -> numeric code.
    Checks SLUG_CODE_OVERRIDES first, then tries name matching.
    """
    result: dict[str, int] = {}
    for p in provinces:
        slug = p["id"]
        # Check manual overrides first
        if slug in SLUG_CODE_OVERRIDES:
            result[slug] = SLUG_CODE_OVERRIDES[slug]
            continue
        name_norm = normalize(p["name"])
        code = province_code_by_name.get(name_norm)
        if code is None:
            # Try with division-type prefixes that appear in map.json names
            for prefix in ("tinh ", "thanh pho trung uong ", "thanh pho "):
                candidate = f"{prefix}{name_norm}"
                code = province_code_by_name.get(candidate)
                if code:
                    break
        if code is None:
            # Try the other direction: strip prefix from map name
            for map_name_norm, map_code in province_code_by_name.items():
                stripped = map_name_norm
                for prefix in ("tinh ", "thanh pho trung uong ", "thanh pho "):
                    if stripped.startswith(prefix):
                        stripped = stripped[len(prefix):]
                        break
                if stripped == name_norm:
                    code = map_code
                    break
        if code is None:
            print(f"  [WARN] No numeric code found for province slug={slug!r}")
        result[slug] = code
    return result


# -------------------------------------------------------------------------
# Enrich province.json
# -------------------------------------------------------------------------

def enrich_province_index(
    provinces: list[dict],
    slug_to_code: dict[str, int],
    province_v2_by_v1code: dict[int, dict],
) -> list[dict]:
    """Add code, status, merged_into to each province entry."""
    enriched = []
    for p in provinces:
        entry = dict(p)
        slug = p["id"]
        code = slug_to_code.get(slug)

        # Insert code right after id
        new_entry: dict = {"id": slug}
        if code is not None:
            new_entry["code"] = code
        new_entry.update({k: v for k, v in entry.items() if k not in ("id",)})

        # Add status if missing
        if "status" not in new_entry:
            new_entry["status"] = "active"

        # Add merged_into if this v1 province was absorbed into a different v2 province
        if code is not None and "merged_into" not in new_entry:
            v2 = province_v2_by_v1code.get(code)
            if v2 and v2["v2_code"] != code:
                new_entry["merged_into"] = str(v2["v2_code"])
                new_entry["status"] = "merged"

        enriched.append(new_entry)
    return enriched


# -------------------------------------------------------------------------
# Enrich per-province address files
# -------------------------------------------------------------------------

def enrich_address_file(
    entries: list[dict],
    province_code: int | None,
    district_code_by_norm: dict[tuple, int],
    ward_code_by_norm: dict[tuple, int],
) -> list[dict]:
    """
    Enrich one per-province address JSON (list of AddressEntry).
    Adds code + status to each district and ward.
    """
    result = []
    for entry in entries:
        new_entry = dict(entry)
        new_districts = []

        for district in entry.get("districts", []):
            d_norm = normalize(district["name"])
            d_code = district_code_by_norm.get((province_code, d_norm)) if province_code else None

            new_district = {}
            new_district["name"] = district["name"]
            new_district["division_type"] = district["division_type"]
            if d_code is not None:
                new_district["code"] = d_code
            if "status" not in district:
                new_district["status"] = "active"
            else:
                new_district["status"] = district["status"]

            new_wards = []
            for ward in district.get("wards", []):
                w_norm = normalize(ward["name"])
                w_code = ward_code_by_norm.get((province_code, d_norm, w_norm)) if province_code else None

                new_ward = {}
                new_ward["name"] = ward["name"]
                new_ward["division_type"] = ward["division_type"]
                if w_code is not None:
                    new_ward["code"] = w_code
                if "status" not in ward:
                    new_ward["status"] = "active"
                else:
                    new_ward["status"] = ward["status"]
                new_ward["details"] = ward.get("details", {"streets": [], "villages_hamlets": []})

                # Preserve any extra fields from original ward
                for k, v in ward.items():
                    if k not in new_ward:
                        new_ward[k] = v

                new_wards.append(new_ward)

            new_district["wards"] = new_wards

            # Preserve any extra fields from original district
            for k, v in district.items():
                if k not in new_district and k != "wards":
                    new_district[k] = v

            new_districts.append(new_district)

        new_entry["districts"] = new_districts
        result.append(new_entry)
    return result


# -------------------------------------------------------------------------
# Main
# -------------------------------------------------------------------------

def main() -> None:
    print("=== Enrich v1 data with code + status fields ===\n")

    map_data = load_json(MAP_FILE)
    (
        province_code_by_name,
        province_v2_by_v1code,
        district_code_by_norm,
        ward_code_by_norm,
        all_v1_codes,
    ) = build_lookups(map_data)

    # --- province.json ---
    province_json_path = V1_DIR / "province.json"
    provinces: list[dict] = load_json(province_json_path)
    slug_to_code = build_province_slug_to_code(provinces, province_code_by_name)

    enriched_provinces = enrich_province_index(provinces, slug_to_code, province_v2_by_v1code)
    save_json(province_json_path, enriched_provinces)

    # --- per-province address files ---
    address_files = [f for f in V1_DIR.glob("*.json") if f.name != "province.json"]
    total_districts = 0
    total_wards = 0
    matched_districts = 0
    matched_wards = 0

    for path in sorted(address_files):
        entries: list[dict] = load_json(path)
        if not entries:
            continue

        # province_id in v1 address files is a numeric string like "01", "79"
        # Parse it directly as integer – these ARE the GSO province codes.
        first_pid = str(entries[0].get("province_id", ""))
        province_code: int | None = None
        try:
            pid_int = int(first_pid)
            if pid_int in all_v1_codes:
                province_code = pid_int
        except ValueError:
            # province_id might already be a slug
            province_code = slug_to_code.get(first_pid)

        if province_code is None:
            print(f"  [WARN] Cannot determine numeric code for {path.name} (province_id={first_pid!r})")
        for entry in entries:
            for d in entry.get("districts", []):
                total_districts += 1
                d_norm = normalize(d["name"])
                if (province_code, d_norm) in district_code_by_norm:
                    matched_districts += 1
                for w in d.get("wards", []):
                    total_wards += 1
                    w_norm = normalize(w["name"])
                    if (province_code, d_norm, w_norm) in ward_code_by_norm:
                        matched_wards += 1

        enriched = enrich_address_file(
            entries, province_code, district_code_by_norm, ward_code_by_norm
        )
        save_json(path, enriched)

    print(f"\nDistricts: {matched_districts}/{total_districts} matched with code")
    print(f"Wards:     {matched_wards}/{total_wards} matched with code")
    print("\nDone! v1 data has been enriched.")


if __name__ == "__main__":
    main()
