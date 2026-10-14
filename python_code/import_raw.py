from app.modules.importer.csv_reader import CsvReader
from app.modules.importer.service import ImporterService

def main():
    rows=CsvReader().parse("../phone_sales.csv")
    result =ImporterService().import_rows("phone_sales.csv",category="phone",rows=rows)
    print(result.batch_id,result.row_count)


if __name__=="__main__":
    main()