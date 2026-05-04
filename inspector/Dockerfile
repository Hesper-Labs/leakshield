# syntax=docker/dockerfile:1.7
FROM python:3.14-slim AS build
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential gcc && \
    rm -rf /var/lib/apt/lists/*
COPY pyproject.toml ./
COPY src ./src
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir .

FROM python:3.14-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r leakshield && useradd -r -g leakshield leakshield
COPY --from=build /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build /usr/local/bin/leakshield-inspector /usr/local/bin/leakshield-inspector
USER leakshield
EXPOSE 50051
ENTRYPOINT ["python", "-m", "leakshield_inspector"]
