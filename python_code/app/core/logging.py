import logging
from contextvars import ContextVar

# 每个请求一个独立的 request_id。ContextVar 是协程/线程隔离的，
# 并发请求之间不会串号。
request_id_var: ContextVar[str] = ContextVar("request_id", default="-")

logging.basicConfig(
    format="%(asctime)s [%(levelname)s] [%(request_id)s] %(message)s",
    level=logging.INFO,
)


class RequestIDFilter(logging.Filter):
    """把当前 ContextVar 里的 request_id 注入到每一条日志记录上。"""

    def filter(self, record: logging.LogRecord) -> bool:
        record.request_id = request_id_var.get()
        return True


logging.getLogger().addFilter(RequestIDFilter())
logger = logging.getLogger(__name__)
