FROM python:3.12-slim AS runtime

WORKDIR /app

ENV PYTHONUNBUFFERED=1 \
    UV_LINK_MODE=copy \
    XDG_CACHE_HOME=/tmp/parser-cache \
    PATH="/app/.venv/bin:${PATH}"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libglib2.0-0 \
        libgl1 \
        libgomp1 \
    && rm -rf /var/lib/apt/lists/*

RUN python -m pip install --no-cache-dir uv==0.11.6

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --extra paddleocr --no-install-project

COPY src ./src
RUN uv sync --frozen --no-dev --extra paddleocr

RUN useradd --system --uid 10001 --create-home parser \
    && chown -R parser:parser /app /tmp/parser-cache
USER parser

EXPOSE 8080

CMD ["uv", "run", "parser-service"]
