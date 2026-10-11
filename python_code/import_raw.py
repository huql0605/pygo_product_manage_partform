import sys
from pathlib import Path

import pandas as pd
import requests

SCRIPT_DIR =Path(__file__).resolve().parent

if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0,str(SCRIPT_DIR))

from app.config import settings

CSV_PATH=SCRIPT_DIR.parent/"phone_sales.csv"
GO_API = settings.go_api_url
SOURCE="phone_sales.csv"
CATEGORY="phone"
def main():
    df = pd.read_csv(CSV_PATH,dtype=str,encoding="utf-8-sig",keep_default_na=False)
    rows=df.to_dict(orient="records")

    resp= requests.post(
        url=f"{GO_API}/api/imports",
        json={"source":SOURCE,"category":CATEGORY,"rows":rows},
        timeout=10
    )

    resp.raise_for_status()

    body = resp.json()
    if body.get("code")!=0:
        raise SystemExit(f"导入失败:{body.get('msg')}")
    print(f"批次{body['data']['batch_id']} 已写入 {body['data']['row_count']}")

if __name__=="__main__":
    main()