import csv
import requests

GO_API = "http://localhost:8080"

def import_product(path :str):
    with open(path ,newline="",encoding="utf-8") as f:
        rows =list(csv.DictReader(f))

    ok=0

    for row in rows:
        resp = requests.post(url=f"{GO_API}/api/products",json={
            "name":row["name"],
            "price":float(row["price"] )      
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
    import_product("products.csv")