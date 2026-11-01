import requests
from dataclasses import dataclass
from app.config import settings
import hashlib
import json


class ImporterInvalidError(Exception):
    """ 上传内容或参数不合法，对应 HTTP 400"""


class ImporterFaildError(Exception):
    """ go 服务不可用， 对应 HTTP 502"""


class ImporterFailedError(Exception):
    """导入失败"""

@dataclass(frozen=True)
class ImportResult:
    batch_id:int
    row_count:int
    duplicated: bool = False
    file_hash: str = ""

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


    def import_file(self,*,source:str,category:str,filename:str,content_type:str,content:bytes,rows:list[dict],reader_version:str,) -> ImportResult:
        file_hash=hashlib.sha256(content).hexdigest()
        files ={
            "file" : (filename,content,content_type or "application/octet-stream"),
        }
        data ={
            "source":source,
            "category":category,
            "file_hash":file_hash,
            "reader_version":reader_version,
            "rows":json.dumps(rows,ensure_ascii=False),
        }

        try:
            resp =requests.post(
                url=f"{self.base_url}/api/import-uploads",
                files=files,
                data=data,
                timeout=self.timeout
            )
            resp.raise_for_status()
        except requests.HTTPError as exc:
            status = exc.response.status_code if exc.response is not None else None
            detail = exc.response.text[:500] if exc.response is not None else str(exc)
            if status is not None and 400 <= status < 500:
                raise ImporterInvalidError(detail) from exc
            raise ImporterFailedError(f"请求 Go 服务失败：{exc}") from exc
        except requests.RequestException as exc:
            raise ImporterFailedError(f"请求 Go 服务失败：{exc}") from exc

        body = resp.json()
        if body.get("code") != 0:
            raise ImporterFaildError(body.get("msg","未知错误"))

        payload = body.get("data") or {}
        return ImportResult(
            batch_id=payload.get("batch_id",0),
            row_count=payload.get("row_count",0),
            duplicated=bool(payload.get("duplicated",False)),
            file_hash=payload.get("file_hash",""),
        )       