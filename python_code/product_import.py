import csv
import requests
import time
from datetime import datetime
GO_API = "http://localhost:8080"

def import_product(path :str):
    with open(path ,newline="",encoding="utf-8-sig") as f:
        rows =list(csv.DictReader(f))

    ok=0

    for row in rows:
        resp = requests.post(url=f"{GO_API}/api/products",json={
            "productor":row["productor"],
            "name":row["name"],
            "price":float(row["price"]),
            "sales":int(row["sales"]) ,
            "month":datetime.strptime(row["month"], "%Y-%m").strftime("%Y-%m-%dT%H:%M:%SZ")
                  
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
    import_product("phone_sales.csv")