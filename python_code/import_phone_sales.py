from pathlib import Path

import pandas as pd 
from sqlalchemy import create_engine

# CSV 文件位于工作区根目录（python_code 的上一级）
CSV_PATH = Path(__file__).resolve().parent.parent / "phone_sales.csv"

engine =create_engine("postgresql+psycopg2://leohu:hu0605@localhost:5432/db1")

df = pd.read_csv(CSV_PATH,encoding="utf-8-sig")

df.columns =[c.strip().lower() for c in df.columns]

df["price"] = df["price"].astype(float)
df["sales"] = df["sales"].astype(int)
df["month"] = df["month"].astype(str).str.strip()

df.to_sql("products",engine,if_exists="append",index=False)

print(f"import {len(df)} rows")
print(df.head())