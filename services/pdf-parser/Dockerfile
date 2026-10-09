ARG PDF_PARSER_BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/misaka-images/python:3.12-slim-bookworm
FROM ${PDF_PARSER_BASE_IMAGE}
ARG PDF_PARSER_APT_MIRROR=https://deb.debian.org
RUN sed -i "s|http://deb.debian.org|${PDF_PARSER_APT_MIRROR}|g" /etc/apt/sources.list.d/debian.sources \
    && apt-get -o Acquire::Retries=2 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 update \
    && apt-get -o Acquire::Retries=2 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 install -y --no-install-recommends \
    poppler-utils poppler-data fonts-noto-cjk qpdf tesseract-ocr tesseract-ocr-chi-sim tesseract-ocr-eng util-linux \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY server.py /app/server.py
ENV PYTHONUNBUFFERED=1 OMP_THREAD_LIMIT=1
USER 10001:10001
EXPOSE 8090
HEALTHCHECK --interval=15s --timeout=5s --retries=3 CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/health', timeout=3)"
CMD ["python", "server.py"]
