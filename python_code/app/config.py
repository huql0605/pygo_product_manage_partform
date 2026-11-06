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
    # 属性聚合允许查询的 key，逗号分隔；设为 * 表示放行任意合法命名的 key
    allowed_attrs:str =Field(default="storage,screen,cpu,ram,color,weight",validation_alias="ALLOWED_ATTRS")

    max_upload_bytes :int =Field(
        default=32*1024*1024,
        validation_alias="MAX_UPLOAD_BYTES",
    )

    import_signing_key :str=Field(validation_alias="IMPORT_SIGNING_KEY")



# 全局唯一实例：字段必须通过实例访问，不能直接用 Settings.database_url
settings = Settings()