from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    database_url: str = "postgresql+psycopg2://leohu:hu0605@localhost:5432/db1"
    go_api_url: str = "http://127.0.0.1:8080"
    class Config:
        env_file = ".env"