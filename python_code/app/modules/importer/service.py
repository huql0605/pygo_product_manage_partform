import requests
from dataclasses import dataclass
from app.config import settings


class ImporterFailedError(Exception):
    """导入失败"""

@dataclass(frozen=True)
class ImportResult:
    batch_id:int
    row_count:int

class ImporterService:
    def __init__(self,base_url:str|None = None , timeout:int = 60) -> None:
        self.base_url=(base_url or settings.go_api_url).rstrip("/")
        self.timeout=timeout


    def import_rows(self,source:str,category:str,rows:list[dict]) ->ImportResult:
        json_data={
            "source":source,
            "category":category,
            "rows":rows,
            }
        try:
            resp = requests.post(
                url=f"{self.base_url}/api/imports",
                json=json_data,
                timeout=self.timeout
                )

            resp.raise_for_status()
        except requests.RequestException as exc:
            raise ImporterFailedError(f"请求 Go 服务失败： {exc}")from exc

        body =resp.json()
        if body.get("code")!=0:
            raise ImporterFailedError(body.get("msg","未知错误"))
        

        data=body.get("data") or {}
        return ImportResult(
            batch_id=data.get("batch_id",0),
            row_count=data.get("row_count",0),
        )
        