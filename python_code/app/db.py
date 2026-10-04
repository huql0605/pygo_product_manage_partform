from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from config import Settings
databaseURL=Settings.database_url

engine =create_engine(databaseURL,pool_pre_ping=True)

SessionLocal=sessionmaker(bind=engine,autoflush=False,autocommit=False)

