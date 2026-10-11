from pydantic_settings import BaseSettings, SettingsConfigDict
from pydantic import Field

class Settings(BaseSettings):

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )
    database_url:str=Field(validation_alias="DATABASE_URL")
    go_api_url:str =Field(validation_alias="GO_API")



# 全局唯一实例：字段必须通过实例访问，不能直接用 Settings.database_url
settings = Settings()