from pathlib import Path

import pandas as pd
from sqlalchemy import create_engine, text

# CSV 文件位于工作区根目录（python_code 的上一级）
BASE_DIR = Path(__file__).resolve().parent.parent
CSV_PATH = BASE_DIR / "phone_sales.csv"

engine = create_engine(
    "postgresql+psycopg2://leohu:hu0605@localhost:5432/db1"
)

# 1. 确保 products 里有 phone
with engine.begin() as conn:
    row = conn.execute(
        text("SELECT id FROM products WHERE name = 'phone'")
    ).fetchone()
    if row:
        phone_id = row[0]
    else:
        result = conn.execute(
            text("INSERT INTO products (name, description) "
                 "VALUES ('phone', '手机') RETURNING id")
        )
        phone_id = result.fetchone()[0]

print("phone product_id =", phone_id)

# 2. 读 CSV
df = pd.read_csv(CSV_PATH, encoding="utf-8-sig")
df.columns = [c.strip().lower() for c in df.columns]
df["price"] = df["price"].astype(float)
df["sales"] = df["sales"].astype(int)
df["month"] = df["month"].astype(str).str.strip()
df["product_id"] = phone_id

# 3. 写入 sales，只保留表里有的列
df = df[["product_id", "productor", "name", "price", "sales", "month"]]
df.to_sql("sales", engine, if_exists="append", index=False)

print(f"imported {len(df)} rows into sales")