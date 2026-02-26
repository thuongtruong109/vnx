#!/usr/bin/env python3
"""
Rebuild /data/v1 inactive province files using data/map.json.

map.json contains the full historical (pre-2025) ward data for all 63 provinces,
grouped under v1_wards in each new (v2) ward entry.

This script:
  - Rebuilds the 29 inactive (merged-away) province files with COMPLETE district
    and ward hierarchy derived from map.json.
  - Skips the 34 active province files (already synced from nested-divisions.json).
  - Preserves existing `details` (streets, villages_hamlets) where present.
  - Marks all rebuilt districts + wards as status: inactive.

Run:
    python scripts/rebuild_v1_inactive_from_map.py
"""

import json
import re
import unicodedata
from pathlib import Path

REPO_ROOT = Path(__file__).parent.parent
DATA_V1 = REPO_ROOT / 'data' / 'v1'
MAP_JSON = REPO_ROOT / 'data' / 'map.json'

# Inactive province slug -> OLD province numeric code (pre-2025)
INACTIVE_SLUG_TO_CODE = {
    'hagiang':       2,
    'backan':        6,
    'laocai':       10,
    'hoabinh':      17,
    'vinhphuc':     26,
    'bacninh':      27,
    'haiduong':     30,
    'thaibinh':     34,
    'hanam':        35,
    'namdinh':      36,
    'quangtri':     45,
    'quangnam':     49,
    'phuyen':       54,
    'ninhthuan':    58,
    'binhthuan':    60,
    'kontum':       62,
    'gialai':       64,
    'daknong':      67,
    'binhphuoc':    70,
    'tayninh':      72,
    'binhduong':    74,
    'baria-vungtau': 77,
    'bentre':       83,
    'travinh':      84,
    'dongthap':     87,
    'angiang':      89,
    'haugiang':     93,
    'soctrang':     94,
    'baclieu':      95,
}

# Infer division_type from Vietnamese name prefix
WARD_PREFIXES = [
    ('Phường ',      'phường'),
    ('Xã ',          'xã'),
    ('Thị trấn ',    'thị trấn'),
    ('Thị xã ',      'thị xã'),
]

DISTRICT_PREFIXES = [
    ('Thành phố ',   'thành phố thuộc tỉnh'),
    ('Thị xã ',      'thị xã'),
    ('Quận ',        'quận'),
    ('Huyện ',       'huyện'),
]


def infer_division_type(name: str, prefixes: list) -> tuple[str, str]:
    """Return (short_name, division_type) by stripping known prefix."""
    for prefix, dtype in prefixes:
        if name.startswith(prefix):
            return name[len(prefix):], dtype
    return name, 'unknown'


def nfc(s: str) -> str:
    return unicodedata.normalize('NFC', s) if s else s


def build_province_index(map_data: dict) -> dict:
    """
    Build index: old_province_code -> {
        'code': int,
        'name': str,
        'division_type': str,
        'districts': {
            district_code: {
                'name': str,
                'division_type': str,
                'wards': {ward_code: {'name': str, 'division_type': str}}
            }
        }
    }
    from map.json v1_wards entries.
    """
    provinces: dict = {}

    for w2 in map_data.get('wards', []):
        for w1 in w2.get('v1_wards', []):
            pc = w1['province_code']
            pname = nfc(w1['province_name'])
            dc = w1['district_code']
            dname = nfc(w1['district_name'])
            wc = w1['code']
            wname = nfc(w1['name'])

            if pc not in provinces:
                _, pdtype = infer_division_type(pname, [
                    ('Thành phố ', 'thành phố trực thuộc trung ương'),
                    ('Tỉnh ', 'tỉnh'),
                ])
                provinces[pc] = {
                    'code': pc,
                    'name': pname,
                    'division_type': pdtype,
                    'districts': {},
                }

            if dc not in provinces[pc]['districts']:
                _, ddtype = infer_division_type(dname, DISTRICT_PREFIXES)
                provinces[pc]['districts'][dc] = {
                    'code': dc,
                    'name': dname,
                    'division_type': ddtype,
                    'wards': {},
                }

            _, wdtype = infer_division_type(wname, WARD_PREFIXES)
            provinces[pc]['districts'][dc]['wards'][wc] = {
                'code': wc,
                'name': wname,
                'division_type': wdtype,
            }

    return provinces


def load_existing_details(filepath: Path) -> dict:
    """Extract existing ward details (streets, villages_hamlets) keyed by ward code."""
    if not filepath.exists():
        return {}
    with open(filepath, encoding='utf-8') as f:
        data = json.load(f)
    details = {}
    for prov in data:
        for dist in prov.get('districts', []):
            for ward in dist.get('wards', []):
                code = ward.get('code')
                d = ward.get('details', {})
                if code and (d.get('streets') or d.get('villages_hamlets')):
                    details[code] = d
    return details


def rebuild_inactive(slug: str, old_code: int, prov_data: dict) -> dict:
    filepath = DATA_V1 / f'{slug}.json'
    existing_details = load_existing_details(filepath)

    province_id = f'{old_code:02d}'

    districts_out = []
    for dc in sorted(prov_data['districts']):
        dist = prov_data['districts'][dc]
        wards_out = []
        for wc in sorted(dist['wards']):
            w = dist['wards'][wc]
            entry = {
                'name': w['name'],
                'division_type': w['division_type'],
                'code': wc,
                'status': 'inactive',
                'details': existing_details.get(wc, {'streets': [], 'villages_hamlets': []}),
            }
            wards_out.append(entry)

        districts_out.append({
            'name': dist['name'],
            'division_type': dist['division_type'],
            'code': dc,
            'status': 'inactive',
            'wards': wards_out,
        })

    new_data = [{
        'province_id': province_id,
        'districts': districts_out,
    }]

    with open(filepath, 'w', encoding='utf-8') as f:
        json.dump(new_data, f, ensure_ascii=False, indent=2)
        f.write('\n')

    total_wards = sum(len(d['wards']) for d in districts_out)
    xa_count = sum(1 for d in districts_out for w in d['wards'] if w['division_type'] == 'xã')
    ph_count = sum(1 for d in districts_out for w in d['wards'] if w['division_type'] == 'phường')
    tt_count = sum(1 for d in districts_out for w in d['wards'] if w['division_type'] == 'thị trấn')

    return {
        'file': slug,
        'districts': len(districts_out),
        'wards': total_wards,
        'xa': xa_count,
        'phuong': ph_count,
        'thi_tran': tt_count,
    }


def main():
    print(f'Loading {MAP_JSON.name} ...')
    with open(MAP_JSON, encoding='utf-8') as f:
        map_data = json.load(f)

    print('Building province index from map.json v1_wards ...')
    prov_index = build_province_index(map_data)
    print(f'  Found {len(prov_index)} old provinces in map.json')

    print(f'\n{"=" * 72}')
    print('Rebuilding 29 inactive province files from map.json')
    print('=' * 72)

    results = []
    for slug in sorted(INACTIVE_SLUG_TO_CODE):
        old_code = INACTIVE_SLUG_TO_CODE[slug]
        prov_data = prov_index.get(old_code)
        if not prov_data:
            print(f'  [ERROR] code={old_code} not found in map.json for slug={slug}')
            continue
        r = rebuild_inactive(slug, old_code, prov_data)
        print(f'  {slug:22s}  code={old_code:3d}  '
              f'dists={r["districts"]:3d}  wards={r["wards"]:4d}  '
              f'(xã={r["xa"]}, phường={r["phuong"]}, thị trấn={r["thi_tran"]})')
        results.append(r)

    total_wards = sum(r['wards'] for r in results)
    print(f'\n{"=" * 72}')
    print(f'XONG. Rebuilt {len(results)} inactive province files.')
    print(f'Total historical wards written: {total_wards}')


if __name__ == '__main__':
    main()
