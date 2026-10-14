import pandas as pd
from pathlib import Path

from typing import IO

class CsvReader:

    def parse(self,source:str|Path|IO[bytes]) ->list[dict]:
        df = pd.read_csv(
            source,
            dtype=str,
            encoding="utf-8-sig",
            keep_default_na=False
            )
        rows =df.to_dict(orient="records")
        return rows
