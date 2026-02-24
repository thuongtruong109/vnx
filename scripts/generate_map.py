"""
Generate data/map.json — the address resolution table.

This file records the relationships between pre-2025 (v1) and post-2025 (v2)
administrative units so the API can answer:
  - "Given an old (v1) province/district/ward, what is the new (v2) equivalent?"
  - "Given a new (v2) province/ward, what were the old (v1) units?"

Schema of map.json
------------------
{
  "metadata": { "generated": "2026-02-24", "source": "vietnam-provinces" },

  "provinces": [
    {
      "v2_code": 1,
      "v2_id": "ha_noi",
      "v2_name": "Thành phố Hà Nội",
      "v1_codes": [1, 8],          // legacy province codes that merged into v2
      "v1_names": ["Hà Nội", "Hòa Bình"]
    },
    ...
  ],

  "wards": [
    {
      "v2_code": 4,
      "v2_name": "Phường Ba Đình",
      "v2_province_code": 1,
      "v1_wards": [
        {
          "code": 4,
          "name": "Phường Trúc Bạch",
          "district_code": 1,
          "district_name": "Quận Ba Đình",
          "province_code": 1,
          "province_name": "Hà Nội"
        },
        ...
      ]
    },
    ...
  ]
}
"""

from __future__ import annotations

import json
from datetime import date
from pathlib import Path

from vietnam_provinces import Province as NewProvince, Ward as NewWard
from vietnam_provinces.legacy import (
    Province as OldProvince,
    District as OldDistrict,
    Ward as OldWard,
)

REPO_ROOT = Path(__file__).parent.parent
OUT_FILE = REPO_ROOT / "data" / "map.json"


def build_province_map() -> list[dict]:
    """Map each v2 province → the v1 provinces it was formed from."""
    entries = []
    for new_prov in NewProvince.iter_all():
        legacy_sources = new_prov.get_legacy_sources()
        v1_codes = [lp.code.value for lp in legacy_sources]
        v1_names = [lp.name for lp in legacy_sources]

        entries.append({
            "v2_code": new_prov.code.value,
            "v2_id": new_prov.codename,
            "v2_name": new_prov.name,
            "v1_codes": v1_codes,
            "v1_names": v1_names,
        })

    return entries


def build_ward_map() -> list[dict]:
    """Map each v2 ward → the v1 wards it was formed from."""
    # Pre-build lookup tables for legacy districts and provinces
    all_old_districts: dict[int, OldDistrict] = {
        d.code.value: d for d in OldDistrict.iter_all()
    }
    all_old_provinces: dict[int, OldProvince] = {
        p.code.value: p for p in OldProvince.iter_all()
    }

    entries = []
    for new_ward in NewWard.iter_all():
        old_sources = new_ward.get_legacy_sources()

        v1_wards = []
        for old_ward in old_sources:
            d_code = old_ward.district_code.value
            district = all_old_districts.get(d_code)
            province = all_old_provinces.get(district.province_code.value) if district else None

            v1_wards.append({
                "code": old_ward.code.value,
                "name": old_ward.name,
                "district_code": d_code,
                "district_name": district.name if district else "",
                "province_code": province.code.value if province else 0,
                "province_name": province.name if province else "",
            })

        entries.append({
            "v2_code": new_ward.code.value,
            "v2_name": new_ward.name,
            "v2_province_code": new_ward.province_code.value,
            "v1_wards": v1_wards,
        })

    return entries


def main() -> None:
    print("Building province map...")
    province_map = build_province_map()
    print(f"  {len(province_map)} new provinces mapped.")

    print("Building ward map...")
    ward_map = build_ward_map()
    print(f"  {len(ward_map)} new wards mapped.")

    out = {
        "metadata": {
            "generated": str(date.today()),
            "source": "vietnam-provinces (PyPI)",
            "description": "v1↔v2 administrative unit resolution table",
        },
        "provinces": province_map,
        "wards": ward_map,
    }

    OUT_FILE.parent.mkdir(parents=True, exist_ok=True)
    with OUT_FILE.open("w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)

    print(f"\nWritten → {OUT_FILE}")


if __name__ == "__main__":
    main()
