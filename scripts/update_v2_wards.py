#!/usr/bin/env python3
"""
Update /data/v2 JSON files based on the official post-merger 2025 database
from https://github.com/foxidigital/database-dia-ban-hanh-chinh-1-7-2025-sau-sat-nhap
"""

import json
import re
import unicodedata
import urllib.request
import os
from pathlib import Path

# Province code → JSON file slug mapping (34 provinces after July 1, 2025 merger)
PROVINCE_FILE_MAP = {
    '01': 'ha_noi',
    '04': 'cao_bang',
    '08': 'tuyen_quang',
    '11': 'dien_bien',
    '12': 'lai_chau',
    '14': 'son_la',
    '15': 'lao_cai',
    '19': 'thai_nguyen',
    '20': 'lang_son',
    '22': 'quang_ninh',
    '24': 'bac_ninh',
    '25': 'phu_tho',
    '31': 'hai_phong',
    '33': 'hung_yen',
    '37': 'ninh_binh',
    '38': 'thanh_hoa',
    '40': 'nghe_an',
    '42': 'ha_tinh',
    '44': 'quang_tri',
    '46': 'hue',
    '48': 'da_nang',
    '51': 'quang_ngai',
    '52': 'gia_lai',
    '56': 'khanh_hoa',
    '66': 'dak_lak',
    '68': 'lam_dong',
    '75': 'dong_nai',
    '79': 'ho_chi_minh',
    '80': 'tay_ninh',
    '82': 'dong_thap',
    '86': 'vinh_long',
    '91': 'an_giang',
    '92': 'can_tho',
    '96': 'ca_mau',
}

DIVISION_TYPE_MAP = {
    'Phường': 'phường',
    'Xã': 'xã',
    'Thị trấn': 'thị trấn',
    'Đặc khu': 'đặc khu',
}

DATA_DIR = Path(__file__).parent.parent / 'data' / 'v2'
WARDS_SQL_URL = 'https://raw.githubusercontent.com/foxidigital/database-dia-ban-hanh-chinh-1-7-2025-sau-sat-nhap/main/wards.sql'


def fetch_sql(url: str) -> str:
    print(f"Fetching {url} ...")
    req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
    with urllib.request.urlopen(req, timeout=60) as resp:
        return resp.read().decode('utf-8')


def normalize(s: str) -> str:
    """Normalize Unicode to NFC for consistent comparison."""
    return unicodedata.normalize('NFC', s) if s else s


def parse_wards_sql(sql: str) -> dict[str, list[dict]]:
    """Parse wards SQL and return {province_code: [{code_int, name, division_type}]}"""
    # Handle escaped single quotes (\') inside string values
    pattern = re.compile(
        r"VALUES\s*\(\d+,\s*'(\d{5})',\s*'((?:[^'\\]|\\.)+)',\s*'(\d{2})'",
        re.MULTILINE
    )
    result: dict[str, list[dict]] = {}
    for m in pattern.finditer(sql):
        ward_code_str = m.group(1)
        ward_name = m.group(2).replace("\\'", "'")  # unescape \'
        prov_code = m.group(3)
        code_int = int(ward_code_str)
        # Normalize Unicode to NFC
        ward_name = normalize(ward_name)
        # Determine division type from name prefix
        division_type = 'xã'  # default
        for prefix, dtype in DIVISION_TYPE_MAP.items():
            if ward_name.startswith(prefix + ' ') or ward_name == prefix:
                division_type = dtype
                break
        if prov_code not in result:
            result[prov_code] = []
        result[prov_code].append({
            'code': code_int,
            'name': ward_name,
            'division_type': division_type,
        })
    return result


def get_province_display_name(province_id: str, prov_code: str) -> tuple[str, str]:
    """Return (display_name, division_type) for province-level district entry."""
    # These are the 6 central-level cities
    city_codes = {'01', '31', '48', '46', '79', '92'}
    if prov_code in city_codes:
        division_type = 'thành phố trung ương'
        prov_name_map = {
            '01': 'Thành phố Hà Nội',
            '31': 'Thành phố Hải Phòng',
            '46': 'Thành phố Huế',
            '48': 'Thành phố Đà Nẵng',
            '79': 'Thành phố Hồ Chí Minh',
            '92': 'Thành phố Cần Thơ',
        }
        return prov_name_map[prov_code], division_type
    else:
        return None, 'tỉnh'


def update_province_json(province_id: str, prov_code: str, sql_wards: list[dict]) -> dict:
    """Load existing JSON, update wards based on SQL data, return stats."""
    filepath = DATA_DIR / f'{province_id}.json'

    if not filepath.exists():
        print(f"  [SKIP] {filepath} does not exist")
        return {'added': 0, 'updated': 0, 'deactivated': 0, 'unchanged': 0}

    with open(filepath, encoding='utf-8') as f:
        data = json.load(f)

    # Build SQL lookup: code_int → ward info
    sql_by_code: dict[int, dict] = {w['code']: w for w in sql_wards}
    sql_codes = set(sql_by_code.keys())

    # Navigate to wards list (flat - single district entry)
    province_obj = data[0]
    district = province_obj['districts'][0]
    existing_wards: list[dict] = district['wards']

    # Build existing lookup
    existing_by_code: dict[int, dict] = {w['code']: w for w in existing_wards}
    existing_codes = set(existing_by_code.keys())

    stats = {'added': 0, 'updated': 0, 'deactivated': 0, 'unchanged': 0}

    # 1. Update existing wards or deactivate removed ones
    for ward in existing_wards:
        code = ward['code']
        if code in sql_by_code:
            sql_ward = sql_by_code[code]
            changed = False
            existing_name_norm = normalize(ward.get('name', ''))
            sql_name_norm = normalize(sql_ward['name'])
            if existing_name_norm != sql_name_norm:
                print(f"    [RENAME] #{code}: '{ward['name']}' → '{sql_ward['name']}'")
                ward['name'] = sql_ward['name']
                changed = True
            elif ward.get('name') != sql_ward['name']:
                # Same logical name but different Unicode form - normalize it
                ward['name'] = sql_ward['name']
                changed = True
            if ward.get('division_type') != sql_ward['division_type']:
                print(f"    [RETYPE] #{code}: '{ward.get('division_type')}' → '{sql_ward['division_type']}'")
                ward['division_type'] = sql_ward['division_type']
                changed = True
            # Ensure active
            if ward.get('status') != 'active':
                ward['status'] = 'active'
                changed = True
            if changed:
                stats['updated'] += 1
            else:
                stats['unchanged'] += 1
        else:
            # Ward no longer exists in new administrative system
            if ward.get('status') != 'inactive':
                print(f"    [DEACTIVATE] #{code}: '{ward.get('name')}'")
                ward['status'] = 'inactive'
                stats['deactivated'] += 1

    # 2. Add new wards from SQL not in existing JSON
    for code in sorted(sql_codes - existing_codes):
        sql_ward = sql_by_code[code]
        new_ward = {
            'name': sql_ward['name'],
            'division_type': sql_ward['division_type'],
            'code': code,
            'details': {
                'streets': [],
                'villages_hamlets': []
            },
            'status': 'active'
        }
        existing_wards.append(new_ward)
        print(f"    [ADD] #{code}: '{sql_ward['name']}'")
        stats['added'] += 1

    # Sort wards by code
    district['wards'] = sorted(existing_wards, key=lambda w: w['code'])

    # Write back
    with open(filepath, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write('\n')

    return stats


def main():
    # Download and parse wards SQL
    sql_content = fetch_sql(WARDS_SQL_URL)
    all_wards = parse_wards_sql(sql_content)
    print(f"\nParsed {sum(len(v) for v in all_wards.values())} wards across {len(all_wards)} provinces\n")

    total_stats = {'added': 0, 'updated': 0, 'deactivated': 0, 'unchanged': 0}

    for prov_code, province_id in sorted(PROVINCE_FILE_MAP.items()):
        wards = all_wards.get(prov_code, [])
        print(f"\n[{prov_code}] {province_id} ({len(wards)} wards in DB):")
        if not wards:
            print(f"  WARNING: No wards found for province code {prov_code}")
            continue
        stats = update_province_json(province_id, prov_code, wards)
        for k, v in stats.items():
            total_stats[k] += v
        print(f"  → Added: {stats['added']}, Updated: {stats['updated']}, "
              f"Deactivated: {stats['deactivated']}, Unchanged: {stats['unchanged']}")

    print(f"\n{'='*60}")
    print(f"TOTAL: Added={total_stats['added']}, Updated={total_stats['updated']}, "
          f"Deactivated={total_stats['deactivated']}, Unchanged={total_stats['unchanged']}")
    print("Done!")


if __name__ == '__main__':
    main()
