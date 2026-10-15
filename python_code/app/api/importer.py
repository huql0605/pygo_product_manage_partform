from pathlib import Path
from fastapi import APIRouter,File,Form,HTTPException,UploadFile

from app.modules.importer.csv_reader import CsvReader

from app.modules.importer.excel_reader import ExcelReader
from app.modules.importer.service import ImporterFailedError,ImporterService


router = APIRouter()

READERS ={
    ".csv":CsvReader,
    ".xls":ExcelReader,
    ".xlsx":ExcelReader
}

@router.post("/upload")
async def upload(
    file: UploadFile = File(...,description="要导入的表格文件"),
    category: str =Form(...,description="品类 "),
):
    suffix = Path(file.filename or "").suffix.lower()
    reader =READERS.get(suffix)
    if reader is None:
        raise HTTPException(
            status_code= 400,
            detail=f"不支持的文件类型 {suffix or '(无扩展名)'}, 目前支持{sorted(READERS)}",
        )

    rows = reader().parse(file.file)
    if not rows:
        raise HTTPException(status_code=400,detail="文件没有数据行")
    try:
        result = ImporterService().import_rows(
            source=file.filename or "upload",
            rows=rows,
            category=category,
        )
    except ImporterFailedError as exc:
        raise HTTPException(status_code=502 , detail=str(exc)) from exc

    return {
        "code": 0,
        "msg": "ok",
        "data": {
            "batch_id": result.batch_id,
            "row_count": result.row_count,
            "category": category,
            "source": file.filename,
        },
    }