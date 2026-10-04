from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from app.config import settings

engine = create_engine(settings.database_url, pool_pre_ping=True)

# SQLAlchemy 2.x 已移除 autocommit 参数
SessionLocal = sessionmaker(bind=engine, autoflush=False)

