import sys
from pathlib import Path

import pandas as pd
import requests

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from app.config import settings

CSV_PATH = SCRIPT_DIR.parent / "phone_sales.csv"
GO_API = settings.go_api_url
PRODUCT = "phone"     # 品类名，必须在 products 表里存在
BATCH_SIZE = 1000

def load_rows(path: Path) -> list[dict]:
    df = pd.read_csv(path, encoding="utf-8-sig")
    df.columns = [c.strip().lower() for c in df.columns]
    df["price"] = df["price"].astype(float)
    df["sales"] = df["sales"].astype(int)
    df["month"] = df["month"].astype(str).str.strip()
    keep = ["productor", "name", "price", "sales", "month"]
    return df[keep].to_dict(orient="records")

def main() -> None:
    rows = load_rows(CSV_PATH)
    total = 0
    for i in range(0, len(rows), BATCH_SIZE):
        chunk = rows[i:i + BATCH_SIZE]
        resp = requests.post(
            f"{GO_API}/api/sales/batch",
            json={"product": PRODUCT, "rows": chunk},
            timeout=30,
        )
        if resp.status_code != 200:
            raise SystemExit(f"批次 {i // BATCH_SIZE} 失败: {resp.status_code} {resp.text}")
        body = resp.json()
        if body.get("code") != 0:
            raise SystemExit(f"批次 {i // BATCH_SIZE} 业务失败: {body.get('msg')}")
        inserted = body["data"]["inserted"]
        total += inserted
        print(f"批次 {i // BATCH_SIZE}: 写入 {inserted} 行")
    print(f"完成: {total}/{len(rows)}")

if __name__ == "__main__":
    main()