import logging
import os
import signal
import time


logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO"),
    format="%(asctime)s %(levelname)s [%(name)s] %(message)s",
)

logger = logging.getLogger("knowledge-worker")
shutdown_requested = False


def handle_shutdown(signum: int, _frame: object) -> None:
    global shutdown_requested
    shutdown_requested = True
    logger.info("received shutdown signal %s", signum)


def main() -> None:
    signal.signal(signal.SIGTERM, handle_shutdown)
    signal.signal(signal.SIGINT, handle_shutdown)

    logger.info("Knowledge Service worker scaffold started")
    logger.info("redis_url=%s", os.getenv("REDIS_URL", "not-configured"))
    logger.info("qdrant_url=%s", os.getenv("QDRANT_URL", "not-configured"))
    logger.info("minio_endpoint=%s", os.getenv("MINIO_ENDPOINT", "not-configured"))

    while not shutdown_requested:
        time.sleep(5)

    logger.info("Knowledge Service worker scaffold stopped")


if __name__ == "__main__":
    main()
