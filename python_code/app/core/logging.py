import logging
import logging.config
from contextvars import ContextVar

# 每个请求一个独立的 request_id。ContextVar 是协程/线程隔离的，
# 并发请求之间不会串号。
request_id_var: ContextVar[str] = ContextVar("request_id", default="-")


class RequestIDFilter(logging.Filter):
    """把当前 ContextVar 里的 request_id 注入到每一条日志记录上。"""

    def filter(self, record: logging.LogRecord) -> bool:
        record.request_id = request_id_var.get()
        return True


# 关键：这里必须用 dictConfig，而不是 basicConfig。
# uvicorn 在导入 app 之前就用自己的 logger/handler 接管了日志（propagate=False），
# basicConfig 只改 root logger，对 uvicorn 的日志完全无效。
# 我们在 app 导入时重新配置，连带覆盖 uvicorn 的 logger，让访问日志也带上 request_id。
LOG_CONFIG = {
    "version": 1,
    "disable_existing_loggers": False,
    "filters": {
        "request_id": {"()": RequestIDFilter},
    },
    "formatters": {
        "default": {
            "()": "uvicorn.logging.DefaultFormatter",
            "fmt": "%(asctime)s [%(levelname)s] [%(request_id)s] %(message)s",
            "use_colors": False,
        },
        "access": {
            "()": "uvicorn.logging.AccessFormatter",
            "fmt": '%(asctime)s [%(levelname)s] [%(request_id)s] %(client_addr)s - "%(request_line)s" %(status_code)s',
            "use_colors": False,
        },
    },
    "handlers": {
        "default": {
            "class": "logging.StreamHandler",
            "formatter": "default",
            "stream": "ext://sys.stderr",
            "filters": ["request_id"],
        },
        "access": {
            "class": "logging.StreamHandler",
            "formatter": "access",
            "stream": "ext://sys.stdout",
            "filters": ["request_id"],
        },
    },
    "loggers": {
        "uvicorn": {"handlers": ["default"], "level": "INFO", "propagate": False},
        "uvicorn.error": {"level": "INFO"},
        "uvicorn.access": {"handlers": ["access"], "level": "INFO", "propagate": False},
    },
    "root": {"handlers": ["default"], "level": "INFO"},
}


logging.config.dictConfig(LOG_CONFIG)

logger = logging.getLogger(__name__)
