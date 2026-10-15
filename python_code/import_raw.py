from app.modules.importer.csv_reader import CsvReader
from app.modules.importer.service import ImporterService
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent
CSV_PATH = BASE_DIR/"phone_sales.csv"
def main():
    rows=CsvReader().parse(CSV_PATH)
    result =ImporterService().import_rows("phone_sales.csv",category="phone",rows=rows)
    print(result.batch_id,result.row_count)


if __name__=="__main__":
    main()