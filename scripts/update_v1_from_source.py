#!/usr/bin/env python3
"""
Update /data/v1 JSON files based on:
  https://github.com/sunshine-tech/VietnamProvinces/blob/main/vietnam_provinces/data/nested-divisions.json

The source has 34 post-merger provinces with wards directly (no districts).
v1 has the old 63-province structure with districts and wards.

Strategy:
  - For each of the 34 new provinces, find the matching v1 file (by province code)
    and REPLACE its content with a single virtual-district containing all new wards.
  - For old province files that were merged away, mark all districts/wards inactive.
"""

import json
import urllib.request
import unicodedata
from pathlib import Path

DATA_V1 = Path('D:/Project/vnx/data/v1')
SOURCE_URL = 'https://raw.githubusercontent.com/sunshine-tech/VietnamProvinces/main/vietnam_provinces/data/nested-divisions.json'

# Map: new province numeric code -> old v1 file slug
# (the old province that retained this code number in the new system)
NEW_CODE_TO_V1_SLUG = {
    1:  'hanoi',          # Hà Nội
    4:  'caobang',        # Cao Bằng
    8:  'tuyenquang',     # Tuyên Quang (absorbed Hà Giang)
    11: 'dienbien',       # Điện Biên
    12: 'laichau',        # Lai Châu
    14: 'sonla',          # Sơn La
    15: 'yenbai',         # Lào Cai (uses old Yên Bái code 15; absorbed old Lào Cai)
    19: 'thainguyen',     # Thái Nguyên (absorbed Bắc Kạn)
    20: 'langson',        # Lạng Sơn
    22: 'quangninh',      # Quảng Ninh
    24: 'bacgiang',       # Bắc Ninh (uses old Bắc Giang code 24; absorbed old Bắc Ninh)
    25: 'phutho',         # Phú Thọ (absorbed Vĩnh Phúc, Hòa Bình)
    31: 'haiphong',       # Hải Phòng (absorbed Hải Dương)
    33: 'hungyen',        # Hưng Yên (absorbed Thái Bình)
    37: 'ninhbinh',       # Ninh Bình (absorbed Hà Nam, Nam Định)
    38: 'thanhhoa',       # Thanh Hóa
    40: 'nghean',         # Nghệ An
    42: 'hatinh',         # Hà Tĩnh
    44: 'quangbinh',      # Quảng Trị (uses old Quảng Bình code 44; absorbed old Quảng Trị)
    46: 'thuathienhue',   # Thành phố Huế (old Thừa Thiên Huế code 46)
    48: 'danang',         # Đà Nẵng (absorbed Quảng Nam)
    51: 'quangngai',      # Quảng Ngãi (absorbed Kon Tum)
    52: 'binhdinh',       # Gia Lai (uses old Bình Định code 52; absorbed old Gia Lai)
    56: 'khanhhoa',       # Khánh Hòa (absorbed Ninh Thuận)
    66: 'daklak',         # Đắk Lắk (absorbed Phú Yên)
    68: 'lamdong',        # Lâm Đồng (absorbed Đắk Nông, Bình Thuận)
    75: 'dongnai',        # Đồng Nai (absorbed Bình Phước)
    79: 'tphcm',          # TP. Hồ Chí Minh (absorbed Bình Dương, Bà Rịa-Vũng Tàu)
    80: 'longan',         # Tây Ninh (uses old Long An code 80; absorbed old Tây Ninh)
    82: 'tiengiang',      # Đồng Tháp (uses old Tiền Giang code 82; absorbed old Đồng Tháp)
    86: 'vinhlong',       # Vĩnh Long (absorbed Bến Tre, Trà Vinh)
    91: 'kiengiang',      # An Giang (uses old Kiên Giang code 91; absorbed old An Giang)
    92: 'cantho',         # Cần Thơ (absorbed Hậu Giang, Sóc Trăng, Bạc Liêu)
    96: 'camau',          # Cà Mau
}

# Old province files that were fully merged away (no longer active in new system)
MERGED_V1_SLUGS = [
    'hagiang',      # merged into tuyenquang (8)
    'backan',       # merged into thainguyen (19)
    'laocai',       # merged into yenbai→lao_cai (15)
    'hoabinh',      # merged into phutho (25)
    'vinhphuc',     # merged into phutho (25)
    'bacninh',      # merged into bacgiang→bac_ninh (24)
    'haiduong',     # merged into haiphong (31)
    'thaibinh',     # merged into hungyen (33)
    'hanam',        # merged into ninhbinh (37)
    'namdinh',      # merged into ninhbinh (37)
    'quangtri',     # merged into quangbinh→quang_tri (44)
    'quangnam',     # merged into danang (48)
    'phuyen',       # merged into daklak (66)
    'ninhthuan',    # merged into khanhhoa (56)
    'binhthuan',    # merged into lamdong (68)
    'kontum',       # merged into quangngai (51)
    'gialai',       # merged into binhdinh→gia_lai (52)
    'daknong',      # merged into lamdong (68)
    'binhphuoc',    # merged into dongnai (75)
    'tayninh',      # merged into longan→tay_ninh (80)
    'binhduong',    # merged into tphcm (79)
    'baria-vungtau',# merged into tphcm (79)
    'bentre',       # merged into vinhlong (86)
    'travinh',      # merged into vinhlong (86)
    'dongthap',     # merged into tiengiang→dong_thap (82)
    'angiang',      # merged into kiengiang→an_giang (91)
    'haugiang',     # merged into cantho (92)
    'soctrang',     # merged into cantho (92)
    'baclieu',      # merged into camau (96)
]


def nfc(s: str) -> str:
    return unicodedata.normalize('NFC', s) if s else s


def fetch_source() -> list:
    print(f'Fetching source data from {SOURCE_URL} ...')
    req = urllib.request.Request(SOURCE_URL, headers={'User-Agent': 'Mozilla/5.0'})
    with urllib.request.urlopen(req, timeout=60) as r:
        data = json.loads(r.read().decode('utf-8'))
    print(f'  Got {len(data)} provinces from source.')
    return data


def build_source_index(source: list) -> dict:
    """Build dict: new_code -> source province object"""
    return {p['code']: p for p in source}


def make_ward_entry(w: dict) -> dict:
    """Convert source ward to v1 ward format."""
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
    Replace the v1 file content with a single virtual-district
    containing all new wards from the source province.
    Returns stats dict.
    """
    filepath = DATA_V1 / f'{slug}.json'
    if not filepath.exists():
        print(f'  [SKIP] {slug}.json not found')
        return {'file': slug, 'action': 'skip'}

    # Determine province_id (zero-padded 2-digit old numeric code = new numeric code)
    province_id = f'{new_code:02d}'

    prov_name = nfc(src_province['name'])
    prov_divtype = nfc(src_province['division_type'])

    # Build wards list
    src_wards = src_province.get('wards', [])
    new_wards = [make_ward_entry(w) for w in src_wards]
    new_wards.sort(key=lambda w: w['code'])

    # Single district = province itself
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
    """
    Mark all districts and wards in a merged province file as inactive.
    Returns stats dict.
    """
    filepath = DATA_V1 / f'{slug}.json'
    if not filepath.exists():
        print(f'  [SKIP] {slug}.json not found')
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
    source = fetch_source()
    src_index = build_source_index(source)

    print(f'\n{"="*60}')
    print('STEP 1: Update 34 active province files with new wards')
    print('="*60}')
    update_stats = []
    for new_code in sorted(NEW_CODE_TO_V1_SLUG):
        slug = NEW_CODE_TO_V1_SLUG[new_code]
        src_prov = src_index.get(new_code)
        if not src_prov:
            print(f'  [ERROR] Source has no province with code {new_code}')
            continue
        result = update_active_province(slug, new_code, src_prov)
        src_name = nfc(src_prov['name'])
        src_codename = src_prov.get('codename', '')
        print(f'  [{new_code:2d}] {slug:20s} → {src_codename:20s}  {result["action"]:10s}  wards={result.get("wards","?")}')
        update_stats.append(result)

    print(f'\n{"="*60}')
    print('STEP 2: Deactivate merged province files (29 old provinces)')
    print('="*60}')
    deact_stats = []
    for slug in sorted(MERGED_V1_SLUGS):
        result = deactivate_merged_province(slug)
        print(f'  {slug:25s}  {result["action"]:20s}  changed={result.get("changed", 0)}')
        deact_stats.append(result)

    updated = sum(1 for s in update_stats if s['action'] == 'updated')
    deactivated = sum(1 for s in deact_stats if s['action'] == 'deactivated')
    total_new_wards = sum(s.get('wards', 0) for s in update_stats if s['action'] == 'updated')

    print(f'\n{"="*60}')
    print(f'DONE.')
    print(f'  Active province files updated : {updated}')
    print(f'  Merged province files deactivated: {deactivated}')
    print(f'  Total new wards written        : {total_new_wards}')


if __name__ == '__main__':
    main()
