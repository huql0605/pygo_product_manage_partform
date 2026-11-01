import hashlib
import io
from pathlib import Path

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from app.config import settings
from app.modules.importer.csv_reader import CsvReader
from app.modules.importer.excel_reader import ExcelReader
from app.modules.importer.service import (
    ImporterFailedError,
    ImporterInvalidError,
    ImporterService,
)

router = APIRouter()

READERS ={
    ".csv":(CsvReader,"csv-v1"),
    ".xls":(ExcelReader,"excel-v1"),
    ".xlsx":(ExcelReader,"excel-v1")
}

@router.post("/upload")
async def upload(
    file: UploadFile = File(...,description="要导入的表格文件"),
    category: str =Form(...,description="品类 "),
):
    suffix = Path(file.filename or "").suffix.lower()
    reader_entry=READERS.get(suffix)
    if reader_entry is None:
        raise HTTPException(
            status_code= 400,
            detail=f"不支持的文件类型 {suffix or '(无扩展名)'}, 目前支持{sorted(READERS)}",
        )
    reader_cls,reader_version=reader_entry

    content=file.file.read(settings.max_upload_bytes+1)
    if not content:
        raise HTTPException(status_code=400,detail="文件没有数据")
    if len(content)>settings.max_upload_bytes:
        raise HTTPException(status_code=400,detail="文件大小超过上限")

    digest = hashlib.sha256(content).hexdigest()
    try:
        rows = reader_cls().parse(io.BytesIO(content))
    except Exception as exc:
        raise HTTPException(status_code=400,detail=f"解析文件失败: {exc}") from exc

    if not rows:        
        raise HTTPException(status_code=400,detail="文件没有数据行")
    if len(rows)>10000:
        raise HTTPException(status_code=400,detail="文件行数超过 10000 行")
    
    try:
        result = ImporterService().import_file(
            source=file.filename or "upload",
            category=category,
            filename=file.filename or "upload",
            content_type=file.content_type or "application/octet-stream",
            content=content,
            rows=rows,
            reader_version=reader_version,
        )
    except ImporterFailedError as exc:
        raise HTTPException(status_code=502 , detail=str(exc)) from exc

    except ImporterInvalidError as exc:
        raise HTTPException(status_code=400,detail=str(exc)) from exc
    
    return {
        "code": 0,
        "msg": "ok",
        "data": {
            "batch_id": result.batch_id,
            "row_count": result.row_count,
            "category": category,
            "source": file.filename,
            "duplicated":result.duplicated,
            "file_hash":result.file_hash or digest
        },
    }