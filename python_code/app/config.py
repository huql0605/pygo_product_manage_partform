from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    database_url: str = "postgresql+psycopg2://leohu:hu0605@localhost:5432/db1"
    go_api_url: str = "http://127.0.0.1:8080"


# 全局唯一实例：字段必须通过实例访问，不能直接用 Settings.database_url
settings = Settings()