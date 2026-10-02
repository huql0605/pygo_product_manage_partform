from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

DATABASE_URL ="postgresql+psycopg2://leohu:hu0605@localhost:5432/db1"

engine =create_engine(DATABASE_URL,pool_pre_ping=True)

SessionLocal=sessionmaker(bind=engine,autoflush=False,autocommit=False)

