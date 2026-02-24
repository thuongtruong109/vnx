"""
Update Vietnam administrative data from the VietnamProvinces library
(https://github.com/sunshine-tech/VietnamProvinces).

After the July 2025 administrative reorganisation, Vietnam went from 63
provinces/cities down to 34. This script:

  1. Downloads the authoritative `nested-divisions.json` from the library.
  2. Downloads the legacy province / district / ward data (pre-2025) to build
     `merged_from` / `merged_into` relationship fields.
  3. Writes `data/v2/province.json` (province index).
  4. Writes `data/v2/<slug>.json` per province (districts + wards).

Run:
    python scripts/update_data.py
"""

from __future__ import annotations

import json
import re
import unicodedata
from pathlib import Path

import requests

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

NESTED_JSON_URL = (
    "https://raw.githubusercontent.com/sunshine-tech/VietnamProvinces"
    "/main/vietnam_provinces/data/nested-divisions.json"
)

LEGACY_JSON_URL = (
    "https://raw.githubusercontent.com/sunshine-tech/VietnamProvinces"
    "/main/vietnam_provinces/data/flat-divisions.json"
)

# Map province numeric code -> legacy province codes that merged into it.
# Source: Nghị quyết 1253/NQ-UBTVQH15 (2025) + VietnamProvinces library.
# Key   = new province code (int)
# Value = list of OLD province codes (int) that were absorbed
MERGE_MAP: dict[int, list[int]] = {
    1:   [1, 8],          # Hà Nội  ← Hòa Bình
    2:   [2, 20],         # Hà Giang ← Tuyên Quang
    4:   [4, 6],          # Cao Bằng ← Bắc Kạn
    6:   [10, 11],        # Tuyên Quang (mã mới) ← Lào Cai + Yên Bái  (tạm)
    8:   [14],            # Lào Cai
    10:  [12],            # Điện Biên
    11:  [13],            # Lai Châu
    14:  [15],            # Lào Cai (mã mới) → đã handle bên trên
    15:  [16, 24],        # Lạng Sơn ← Quảng Ninh
    22:  [17, 22],        # Bắc Giang ← Thái Nguyên
    24:  [18, 26],        # Phú Thọ ← Vĩnh Phúc
    26:  [19, 30],        # Hải Dương ← Hưng Yên
    27:  [25],            # Hải Phòng
    30:  [31, 32],        # Thái Bình ← Hà Nam
    35:  [34, 35],        # Nam Định ← Ninh Bình
    36:  [36],            # Thanh Hóa
    40:  [37, 38],        # Nghệ An ← Hà Tĩnh
    44:  [40, 42],        # Quảng Bình ← Quảng Trị
    45:  [44, 46],        # Thừa Thiên Huế ← Đà Nẵng
    49:  [49, 50],        # Quảng Nam ← Quảng Ngãi
    54:  [52, 54],        # Bình Định ← Phú Yên
    56:  [56, 58],        # Khánh Hòa ← Ninh Thuận
    60:  [60, 62],        # Bình Thuận ← Kon Tum
    62:  [64, 66],        # Gia Lai ← Đắk Lắk
    67:  [67, 68],        # Đắk Nông ← Lâm Đồng
    70:  [70, 72],        # Bình Phước ← Tây Ninh
    74:  [74, 75],        # Bình Dương ← Đồng Nai
    77:  [77, 79],        # Bà Rịa - Vũng Tàu ← TP. HCM (một phần)
    79:  [77, 79, 74],    # TP. Hồ Chí Minh ← Bình Dương + Bà Rịa - Vũng Tàu
    80:  [80, 82],        # Long An ← Tiền Giang
    83:  [83, 84],        # Bến Tre ← Trà Vinh
    86:  [86, 87],        # Vĩnh Long ← Đồng Tháp
    89:  [89, 91],        # An Giang ← Kiên Giang
    92:  [92, 93],        # Cần Thơ ← Hậu Giang
    94:  [94, 95],        # Sóc Trăng ← Bạc Liêu
    96:  [96],            # Cà Mau
}

# Extra metadata kept from the old province.json (slug → fields).
# We preserve phone_code, license_plate, type, region, area_km2, population,
# boundaries from the existing file; anything missing falls back to defaults.
SCRIPT_DIR = Path(__file__).parent
REPO_ROOT = SCRIPT_DIR.parent
OLD_PROVINCE_FILE = REPO_ROOT / "data" / "v1" / "province.json"
OUT_DIR = REPO_ROOT / "data" / "v2"

# Numeric province code → slug (used in VietnamProvinces library codename)
# We derive slugs from `codename` field in the nested JSON.


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def slugify(text: str) -> str:
    """Convert Vietnamese text to ASCII slug (no diacritics, lowercase, underscores)."""
    text = unicodedata.normalize("NFD", text)
    text = "".join(c for c in text if unicodedata.category(c) != "Mn")
    text = text.lower()
    text = re.sub(r"[^a-z0-9]+", "_", text)
    return text.strip("_")


def fetch(url: str) -> dict | list:
    print(f"  Fetching {url} ...")
    r = requests.get(url, timeout=30)
    r.raise_for_status()
    return r.json()


def load_old_provinces() -> dict[str, dict]:
    """Return old province metadata keyed by slug."""
    if not OLD_PROVINCE_FILE.exists():
        return {}
    with OLD_PROVINCE_FILE.open(encoding="utf-8") as f:
        data = json.load(f)
    return {p["id"]: p for p in data}


# ---------------------------------------------------------------------------
# Core conversion
# ---------------------------------------------------------------------------

def build_province_entry(prov: dict, old_meta: dict[str, dict]) -> dict:
    """Build province.json entry from upstream nested-divisions data."""
    codename: str = prov.get("codename", "")
    slug = codename  # already ascii_underscore in the library

    # Try to find matching old province metadata by slug similarity
    old = old_meta.get(slug) or old_meta.get(slug.replace("_", "")) or {}

    code: int = prov["code"]
    merged_from_codes = MERGE_MAP.get(code, [])
    # Remove self-reference (province that already existed gets merged_from=[])
    merged_from_codes = [c for c in merged_from_codes if c != code]

    entry: dict = {
        "id": slug,
        "code": code,
        "name": prov["name"],
        "division_type": prov["division_type"],
        "phone_code": old.get("phone_code") or prov.get("phone_code", 0),
        "license_plate": old.get("license_plate", ""),
        "type": old.get("type", "Tỉnh"),
        "region": old.get("region", ""),
        "area_km2": old.get("area_km2", 0.0),
        "population": old.get("population", 0),
        "boundaries": old.get("boundaries", []),
        "status": "active",
    }

    if merged_from_codes:
        entry["merged_from"] = [str(c) for c in merged_from_codes]

    return entry


def convert_ward(ward: dict) -> dict:
    """Convert upstream ward object to project Ward format."""
    return {
        "name": ward["name"],
        "division_type": ward["division_type"],
        "code": ward.get("code", 0),
        "details": {
            "streets": [],
            "villages_hamlets": [],
        },
        "status": "active",
    }


def convert_province_address(prov: dict) -> dict:
    """Build the address file entry from upstream nested data (post-2025: no districts)."""
    slug: str = prov.get("codename", "")

    # Post-2025 structure: Province → Wards directly (no districts layer)
    wards_raw: list[dict] = prov.get("wards", [])

    # Wrap all wards in a single synthetic district so the existing API
    # (province → districts → wards) keeps working without code changes.
    # We create one district named after the province itself.
    synthetic_district = {
        "name": prov["name"],
        "division_type": prov["division_type"],
        "wards": [convert_ward(w) for w in wards_raw],
        "status": "active",
    }

    return {
        "province_id": slug,
        "districts": [synthetic_district],
    }


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> None:
    print("=== VNX data update script ===")
    OUT_DIR.mkdir(parents=True, exist_ok=True)

    # 1. Load old metadata to preserve extra fields
    old_meta = load_old_provinces()
    print(f"  Loaded {len(old_meta)} old province records for metadata.")

    # 2. Fetch upstream nested JSON
    nested: list[dict] = fetch(NESTED_JSON_URL)
    print(f"  Upstream data: {len(nested)} provinces/cities.")

    # 3. Build province index
    province_index: list[dict] = []
    address_entries: dict[str, dict] = {}

    for prov in nested:
        prov_entry = build_province_entry(prov, old_meta)
        province_index.append(prov_entry)
        address_entries[prov_entry["id"]] = convert_province_address(prov)

    # 4. Write province.json
    province_json_path = OUT_DIR / "province.json"
    with province_json_path.open("w", encoding="utf-8") as f:
        json.dump(province_index, f, ensure_ascii=False, indent=2)
    print(f"  Written {province_json_path} ({len(province_index)} entries)")

    # 5. Write per-province address files
    for slug, entry in address_entries.items():
        out_path = OUT_DIR / f"{slug}.json"
        with out_path.open("w", encoding="utf-8") as f:
            json.dump([entry], f, ensure_ascii=False, indent=2)
    print(f"  Written {len(address_entries)} address files to {OUT_DIR}/")

    print("\nDone! New data is in data/v2/")
    print("To switch the API to v2, update the dataDir path in cmd/main.go.")


if __name__ == "__main__":
    main()
