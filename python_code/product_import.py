import csv
import sys
import time
from datetime import datetime
from pathlib import Path

import requests

# 保证从任意工作目录运行都能找到 app 包（python_code 目录）
SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

# CSV 文件位于工作区根目录（python_code 的上一级）
CSV_PATH = SCRIPT_DIR.parent / "products.csv"

from app.config import settings

GO_API = settings.go_api_url

def import_product(path :str):
    with open(path ,newline="",encoding="utf-8-sig") as f:
        rows =list(csv.DictReader(f))

    ok=0

    for row in rows:
        resp = requests.post(url=f"{GO_API}/api/products",json={
            "name": row["name"],
            "price": float(row["price"])
                  
        })
        if resp.status_code==201:
            ok+=1
            print(f"ok {row['name']}")
        else:
            print(f"err {row['name']} : {resp.text}")
    print(f"\n{ok}/{len(rows)} imported")

    r = requests.get(f"{GO_API}/api/products")

    print("current product : ",r.json)

if __name__=="__main__":
    import_product(str(CSV_PATH))