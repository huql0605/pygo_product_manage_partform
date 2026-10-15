import pandas as pd

class ExcelReader:
    def parse(self,source)->list[dict]:
        df = pd.read_excel(source,dtype=str,keep_default_na=False)
        return df.to_dict(orient="records")