#!/usr/bin/env python3
"""
Sync /data/v1 JSON files from local data/nested-divisions.json.

Dùng file nested-divisions.json (34 tỉnh/thành sau sáp nhập 2025) để:
  1. Cập nhật wards cho 34 tỉnh còn hoạt động (active).
  2. Đánh dấu inactive toàn bộ quận/huyện và xã/phường
     trong các tỉnh đã bị sáp nhập.

Chạy:
    python scripts/sync_v1_from_local.py
"""

import json
import unicodedata
from pathlib import Path

REPO_ROOT = Path(__file__).parent.parent
DATA_V1 = REPO_ROOT / 'data' / 'v1'
NESTED_JSON = REPO_ROOT / 'data' / 'nested-divisions.json'

# Map: new province numeric code -> old v1 file slug
NEW_CODE_TO_V1_SLUG = {
    1:  'hanoi',
    4:  'caobang',
    8:  'tuyenquang',
    11: 'dienbien',
    12: 'laichau',
    14: 'sonla',
    15: 'yenbai',
    19: 'thainguyen',
    20: 'langson',
    22: 'quangninh',
    24: 'bacgiang',
    25: 'phutho',
    31: 'haiphong',
    33: 'hungyen',
    37: 'ninhbinh',
    38: 'thanhhoa',
    40: 'nghean',
    42: 'hatinh',
    44: 'quangbinh',
    46: 'thuathienhue',
    48: 'danang',
    51: 'quangngai',
    52: 'binhdinh',
    56: 'khanhhoa',
    66: 'daklak',
    68: 'lamdong',
    75: 'dongnai',
    79: 'tphcm',
    80: 'longan',
    82: 'tiengiang',
    86: 'vinhlong',
    91: 'kiengiang',
    92: 'cantho',
    96: 'camau',
}

# Các tỉnh cũ đã bị sáp nhập hoàn toàn
MERGED_V1_SLUGS = [
    'hagiang',
    'backan',
    'laocai',
    'hoabinh',
    'vinhphuc',
    'bacninh',
    'haiduong',
    'thaibinh',
    'hanam',
    'namdinh',
    'quangtri',
    'quangnam',
    'phuyen',
    'ninhthuan',
    'binhthuan',
    'kontum',
    'gialai',
    'daknong',
    'binhphuoc',
    'tayninh',
    'binhduong',
    'baria-vungtau',
    'bentre',
    'travinh',
    'dongthap',
    'angiang',
    'haugiang',
    'soctrang',
    'baclieu',
]


def nfc(s: str) -> str:
    return unicodedata.normalize('NFC', s) if s else s


def load_nested() -> list:
    with open(NESTED_JSON, encoding='utf-8') as f:
        data = json.load(f)
    print(f'Đã đọc {len(data)} tỉnh/thành từ {NESTED_JSON.name}')
    return data


def build_source_index(source: list) -> dict:
    return {p['code']: p for p in source}


def make_ward_entry(w: dict) -> dict:
    return {
        'name': nfc(w['name']),
        'division_type': nfc(w['division_type']),
        'code': w['code'],
        'status': 'active',
        'details': {
            'streets': [],
            'villages_hamlets': []
        }
    }


def update_active_province(slug: str, new_code: int, src_province: dict) -> dict:
    """
    Ghi lại file v1 với 1 virtual-district chứa toàn bộ wards mới.
    Giữ nguyên details (streets, villages_hamlets) nếu ward đã tồn tại (match theo code).
    """
    filepath = DATA_V1 / f'{slug}.json'
    if not filepath.exists():
        print(f'  [SKIP] {slug}.json không tìm thấy')
        return {'file': slug, 'action': 'skip'}

    # Đọc file cũ để giữ lại details nếu có
    with open(filepath, encoding='utf-8') as f:
        old_data = json.load(f)

    # Build index: ward_code -> existing details
    old_details: dict[int, dict] = {}
    for prov_obj in old_data:
        for district in prov_obj.get('districts', []):
            for ward in district.get('wards', []):
                code = ward.get('code')
                details = ward.get('details', {'streets': [], 'villages_hamlets': []})
                if code and (details.get('streets') or details.get('villages_hamlets')):
                    old_details[code] = details

    province_id = f'{new_code:02d}'
    prov_name = nfc(src_province['name'])
    prov_divtype = nfc(src_province['division_type'])

    src_wards = src_province.get('wards', [])
    new_wards = []
    for w in src_wards:
        entry = make_ward_entry(w)
        # Giữ lại details cũ nếu có
        if w['code'] in old_details:
            entry['details'] = old_details[w['code']]
        new_wards.append(entry)
    new_wards.sort(key=lambda w: w['code'])

    district = {
        'name': prov_name,
        'division_type': prov_divtype,
        'code': new_code,
        'status': 'active',
        'wards': new_wards
    }

    new_data = [{
        'province_id': province_id,
        'districts': [district]
    }]

    with open(filepath, 'w', encoding='utf-8') as f:
        json.dump(new_data, f, ensure_ascii=False, indent=2)
        f.write('\n')

    return {'file': slug, 'action': 'updated', 'wards': len(new_wards)}


def deactivate_merged_province(slug: str) -> dict:
    filepath = DATA_V1 / f'{slug}.json'
    if not filepath.exists():
        print(f'  [SKIP] {slug}.json không tìm thấy')
        return {'file': slug, 'action': 'skip'}

    with open(filepath, encoding='utf-8') as f:
        data = json.load(f)

    changed = 0
    for province_obj in data:
        for district in province_obj.get('districts', []):
            if district.get('status') != 'inactive':
                district['status'] = 'inactive'
                changed += 1
            for ward in district.get('wards', []):
                if ward.get('status') != 'inactive':
                    ward['status'] = 'inactive'
                    changed += 1

    if changed:
        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
            f.write('\n')

    return {'file': slug, 'action': 'deactivated' if changed else 'already_inactive', 'changed': changed}


def main():
    source = load_nested()
    src_index = build_source_index(source)

    print(f'\n{"=" * 65}')
    print('BƯỚC 1: Cập nhật 34 tỉnh/thành active với wards mới')
    print('=' * 65)
    update_stats = []
    for new_code in sorted(NEW_CODE_TO_V1_SLUG):
        slug = NEW_CODE_TO_V1_SLUG[new_code]
        src_prov = src_index.get(new_code)
        if not src_prov:
            print(f'  [LỖI] Không tìm thấy tỉnh code={new_code} trong nguồn')
            continue
        result = update_active_province(slug, new_code, src_prov)
        src_name = nfc(src_prov['name'])
        print(f'  [{new_code:2d}] {slug:22s}  {result["action"]:10s}  wards={result.get("wards", "?")}   ({src_name})')
        update_stats.append(result)

    print(f'\n{"=" * 65}')
    print('BƯỚC 2: Đánh dấu inactive các tỉnh đã sáp nhập')
    print('=' * 65)
    deact_stats = []
    for slug in sorted(MERGED_V1_SLUGS):
        result = deactivate_merged_province(slug)
        print(f'  {slug:25s}  {result["action"]:20s}  changed={result.get("changed", 0)}')
        deact_stats.append(result)

    updated = sum(1 for s in update_stats if s['action'] == 'updated')
    deactivated = sum(1 for s in deact_stats if s['action'] == 'deactivated')
    already_inactive = sum(1 for s in deact_stats if s['action'] == 'already_inactive')
    total_new_wards = sum(s.get('wards', 0) for s in update_stats if s['action'] == 'updated')

    print(f'\n{"=" * 65}')
    print('XONG.')
    print(f'  Tỉnh active cập nhật         : {updated}')
    print(f'  Tỉnh sáp nhập đánh inactive  : {deactivated}')
    print(f'  Tỉnh sáp nhập đã inactive    : {already_inactive}')
    print(f'  Tổng số wards mới            : {total_new_wards}')


if __name__ == '__main__':
    main()
