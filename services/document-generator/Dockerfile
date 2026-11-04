ARG DOCUMENT_GENERATOR_BASE_IMAGE=python:3.12-slim-bookworm
FROM ${DOCUMENT_GENERATOR_BASE_IMAGE}
ARG PIP_INDEX_URL=https://pypi.org/simple
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt && apt-get update && apt-get install -y --no-install-recommends fonts-noto-cjk && rm -rf /var/lib/apt/lists/*
COPY server.py office_style.py ./
USER 65534:65534
EXPOSE 8091
HEALTHCHECK --interval=30s --timeout=3s CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8091/health', timeout=2)"
CMD ["python", "server.py"]
