#!/usr/bin/env python3
import sys
import json

data = json.loads(sys.stdin.read())
print(f'Found {len(data["items"])} products:')
for p in data['items']:
    if p.get('prices') and len(p['prices']) > 0:
        price_data = p['prices'][0]
        price = price_data.get('price_amount', 0) / 100 if 'price_amount' in price_data else 0
    else:
        price = 0
    archived = " [ARCHIVED]" if p.get('is_archived') else ""
    print(f'  - {p["name"]} (${price:.2f}/{p.get("recurring_interval", "?")}) {archived}')